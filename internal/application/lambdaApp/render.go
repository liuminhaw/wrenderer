package lambdaApp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/cmd/shared"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
)

type Application struct {
	Logger *slog.Logger
}

const (
	JobKeyLength = 6

	timestampFile = "timestamp"
)

// RenderUrl will check if the given url is already rendered and cached in S3 bucket.
// If not, it will render the url and upload the result to S3 bucket for caching.
// existenceCheck is a flag to check the existence of the object in S3 bucket.
// If the flag is set to false, the object will be rendered and uploaded to S3 bucket
// no matter if the object already exists in the bucket.
// The cached object path will be returned if no error occurred, otherwise an error
// will be returned.
func (app *Application) RenderUrl(url string, existenceCheck bool) (string, error) {
	envConf, err := shared.LambdaReadEnv()
	if err != nil {
		return "", err
	}

	// Check if object exists
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}
	client := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(url, wrender.CachedPagePrefix)
	if err != nil {
		return "", err
	}
	caching := wrender.NewS3Caching(
		client,
		render.GetPrefixPath(),
		render.CachePath,
		wrender.S3CachingMeta{
			Bucket:      envConf.S3BucketName,
			Region:      envConf.S3BucketRegion,
			ContentType: wrender.HtmlContentType,
		},
	)

	// var exists bool
	if existenceCheck {
		exists, err := caching.Exists()
		if err != nil {
			return "", err
		}
		if exists {
			return caching.CachedPath, nil
		}
	}

	// Render the page
	content, err := app.renderPage(url)
	if err != nil {
		return "", err
	}

	// Check if rendered result is empty
	if len(content) == 0 {
		return "", fmt.Errorf("empty content render result")
	}

	// Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	if err := caching.Update(contentReader); err != nil {
		return "", err
	}

	return caching.CachedPath, nil
}

func (app *Application) RenderSitemap(url string) (string, error) {
	envConf, err := shared.LambdaReadEnv()
	if err != nil {
		return "", err
	}

	entries, err := internal.ParseSitemap(url)
	if err != nil {
		return "", err
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}

	sqsClient := sqs.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	randomKey, err := wrender.RandomKey(JobKeyLength, JobKeyLength)
	if err != nil {
		return "", err
	}

	// Upload render timestamp to S3
	jobCache := wrender.NewSqsJobCache(
		randomKey,
		internal.SitemapCategory,
		wrender.CachedJobPrefix,
	)
	caching := wrender.NewS3Caching(
		s3Client,
		jobCache.KeyPath(),
		"",
		wrender.S3CachingMeta{
			Bucket:      envConf.S3BucketName,
			Region:      envConf.S3BucketRegion,
			ContentType: wrender.PlainContentType,
		},
	)

	now := time.Now().UTC().Format(time.RFC3339)
	if err := caching.UpdateTo(bytes.NewReader([]byte(now)), timestampFile); err != nil {
		return "", err
	}

	queue := internal.Queue{
		Client: sqsClient,
		Url:    envConf.SqsUrl,
	}
	for _, entry := range entries {
		app.Logger.Debug(fmt.Sprintf("Entry: %s", entry.Loc))
		payload, err := json.Marshal(
			wrender.SqsJobPayload{TargetUrl: entry.Loc, RandomKey: randomKey},
		)
		if err != nil {
			return "", err
		}

		messageId, err := queue.SendMessage(string(payload))
		if err != nil {
			return "", err
		}
		app.Logger.Debug(
			fmt.Sprintf("Message id %s successfully sent", messageId),
			slog.String("payload", string(payload)),
		)

		suffixPath := fmt.Sprintf("%s/%s", internal.JobStatusQueued, messageId)
		if err := caching.UpdateTo(bytes.NewReader(payload), suffixPath); err != nil {
			return "", err
		}
	}

	return randomKey, nil
}

func (app *Application) DeleteUrlRenderCache(url string) error {
	envConfig, err := shared.LambdaReadEnv()
	if err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(url, wrender.CachedPagePrefix)
	if err != nil {
		return err
	}
	caching := wrender.NewS3Caching(
		s3Client,
		render.GetPrefixPath(),
		render.CachePath,
		wrender.S3CachingMeta{
			Bucket:      envConfig.S3BucketName,
			Region:      envConfig.S3BucketRegion,
			ContentType: wrender.HtmlContentType,
		},
	)

	// Remove the object from s3
	if err := caching.Delete(); err != nil {
		return err
	}
	empty, err := caching.IsEmptyPrefix("")
	if err != nil {
		return err
	}
	if empty {
		return caching.DeletePrefix()
	}

	return nil
}

func (app *Application) DeleteDomainRenderCache(domain string) error {
	envConfig, err := shared.LambdaReadEnv()
	if err != nil {
		return err
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	s3Client := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(domain, wrender.CachedPagePrefix)
	if err != nil {
		return err
	}
	caching := wrender.NewS3Caching(
		s3Client,
		render.GetPrefixPath(),
		render.CachePath,
		wrender.S3CachingMeta{
			Bucket:      envConfig.S3BucketName,
			Region:      envConfig.S3BucketRegion,
			ContentType: wrender.HtmlContentType,
		},
	)

	return caching.DeletePrefix()
}

func (app *Application) CheckRenderStatus(key string) (shared.RenderStatusResp, error) {
	envConf, err := shared.LambdaReadEnv()
	if err != nil {
		return shared.RenderStatusResp{}, err
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return shared.RenderStatusResp{}, err
	}
	client := s3.NewFromConfig(cfg)

	jobCache := wrender.NewSqsJobCache(key, internal.SitemapCategory, wrender.CachedJobPrefix)
	caching := wrender.NewS3Caching(
		client,
		jobCache.KeyPath(),
		filepath.Join(jobCache.KeyPath(), timestampFile),
		wrender.S3CachingMeta{
			Bucket:      envConf.S3BucketName,
			Region:      envConf.S3BucketRegion,
			ContentType: wrender.PlainContentType,
		},
	)

	queueEmpty, err := caching.IsEmptyPrefix(internal.JobStatusQueued)
	if err != nil {
		return shared.RenderStatusResp{}, err
	}
	processEmpty, err := caching.IsEmptyPrefix(internal.JobStatusProcessing)
	if err != nil {
	}
	failureEmpty, err := caching.IsEmptyPrefix(internal.JobStatusFailed)
	if err != nil {
		return shared.RenderStatusResp{}, err
	}

	// Read job timestamp record
	now := time.Now().UTC()
	timestamp, err := caching.Read()
	if err != nil {
		return shared.RenderStatusResp{}, err
	}
	parsedTime, err := time.Parse(time.RFC3339, string(timestamp))
	if err != nil {
		return shared.RenderStatusResp{}, err
	}
	if now.Sub(parsedTime) > time.Duration(envConf.JobExpirationInHours)*time.Hour {
		return shared.RenderStatusResp{Status: internal.JobStatusTimeout}, nil
	}

	if !queueEmpty || !processEmpty {
		return shared.RenderStatusResp{Status: internal.JobStatusProcessing}, nil
	} else if !failureEmpty {
		failureResp := shared.RenderStatusResp{Status: internal.JobStatusFailed, Details: []string{}}
		failureContents, err := caching.List(internal.JobStatusFailed)
		if err != nil {
			return shared.RenderStatusResp{}, err
		}
		for _, content := range failureContents {
			app.Logger.Debug(fmt.Sprintf("Failure object key: %s", content.Path))

			var queuePayload wrender.SqsJobPayload
			if err := json.Unmarshal(content.Content, &queuePayload); err != nil {
				return shared.RenderStatusResp{}, err
			}
			failureResp.Details = append(failureResp.Details, queuePayload.TargetUrl)
		}

		return failureResp, nil
	}

	return shared.RenderStatusResp{Status: internal.JobStatusCompleted}, nil
}

func (app *Application) renderPage(urlParam string) ([]byte, error) {
	idleType, exists := os.LookupEnv("WRENDERER_IDLE_TYPE")
	if !exists {
		idleType = "networkIdle"
	}

	var windowWidth, windowHeight int
	var err error
	windowWidthConfig, exists := os.LookupEnv("WRENDERER_WINDOW_WIDTH")
	if !exists {
		windowWidth = 1920
	} else {
		windowWidth, err = strconv.Atoi(windowWidthConfig)
		if err != nil {
			return nil, fmt.Errorf("renderPage: %w", err)
		}
	}
	windowHeightConfig, exists := os.LookupEnv("WRENDERER_WINDOW_HEIGHT")
	if !exists {
		windowHeight = 1080
	} else {
		windowHeight, err = strconv.Atoi(windowHeightConfig)
		if err != nil {
			return nil, fmt.Errorf("renderPage: %w", err)
		}
	}

	userAgent, exists := os.LookupEnv("WRENDERER_USER_AGENT")
	if !exists {
		userAgent = ""
	}

	r := renderer.NewRenderer(renderer.WithLogger(app.Logger))
	content, err := r.RenderPage(urlParam, &renderer.RendererOption{
		BrowserOpts: renderer.BrowserConf{
			IdleType:  idleType,
			Container: true,
		},
		Headless:     true,
		WindowWidth:  windowWidth,
		WindowHeight: windowHeight,
		Timeout:      30,
		UserAgent:    userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("renderPage: %w", err)
	}

	return content, nil
}
