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
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
)

type Application struct {
	Logger *slog.Logger
}

const (
	JobKeyLength = 6

	SitemapCategory = "sitemap"

	HtmlContentType  = "text/html"
	PlainContentType = "text/plain"

	timestampFile = "timestamp"

	jobStatusUnknown    = "unknown"
	jobStatusQueued     = "queued"
	jobStatusFailed     = "failed"
	jobStatusProcessing = "processing"
	jobStatusCompleted  = "completed"
	jobStatusTimeout    = "timeout"
)

// RenderUrl will check if the given url is already rendered and cached in S3 bucket.
// If not, it will render the url and upload the result to S3 bucket for caching.
// existenceCheck is a flag to check the existence of the object in S3 bucket.
// If the flag is set to false, the object will be rendered and uploaded to S3 bucket
// no matter if the object already exists in the bucket.
// The cached object path will be returned if no error occurred, otherwise an error
// will be returned.
func (app *Application) RenderUrl(url string, existenceCheck bool) (string, error) {
	render, err := wrender.NewWrender(url, wrender.CachedPagePrefix)
	if err != nil {
		return "", err
	}

	// Check if object exists
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}
	client := s3.NewFromConfig(cfg)

	if existenceCheck {
		exists, err := CheckObjectExists(client, render.CachePath)
		if err != nil {
			return "", err
		}
		if exists {
			return render.CachePath, nil
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
	err = UploadToS3(client, render.CachePath, HtmlContentType, contentReader)
	if err != nil {
		return "", err
	}

	return render.CachePath, nil
}

type workerQueuePayload struct {
	TargetUrl string `json:"targetUrl"`
	CacheKey  string `json:"cacheKey"`
}

func (app *Application) RenderSitemap(url string) (string, error) {
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

	renderKey, err := wrender.RandomKey(JobKeyLength, JobKeyLength)
	if err != nil {
		return "", err
	}

	// Upload render timestamp to S3
	jobCache := wrender.NewSqsJobCache("", renderKey, SitemapCategory)
	now := time.Now().UTC().Format(time.RFC3339)
	if err := UploadToS3(
		s3Client,
		filepath.Join(jobCache.KeyPath(), timestampFile),
		PlainContentType,
		bytes.NewReader([]byte(now)),
	); err != nil {
		return "", err
	}

	for _, entry := range entries {
		app.Logger.Debug(fmt.Sprintf("Entry: %s", entry.Loc))
		payload, err := json.Marshal(workerQueuePayload{TargetUrl: entry.Loc, CacheKey: renderKey})
		if err != nil {
			return "", err
		}

		messageId, err := sendMessageToQueue(sqsClient, string(payload))
		if err != nil {
			return "", err
		}
		app.Logger.Debug(
			fmt.Sprintf("Message id %s successfully sent", messageId),
			slog.String("payload", string(payload)),
		)

		// jobCache := wrender.NewSqsJobCache(messageId, renderKey, SitemapCategory)
		jobCache.MessageId = messageId
		if err := UploadToS3(s3Client, jobCache.QueuedPath(), PlainContentType, bytes.NewReader(payload)); err != nil {
			return "", err
		}
	}

	return renderKey, nil
}

func (app *Application) DeleteUrlRenderCache(url string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	sqsClient := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(url, wrender.CachedPagePrefix)
	if err != nil {
		return err
	}

	// Remove the object from s3
	if err := DeleteObjectFromS3(sqsClient, render.CachePath); err != nil {
		return err
	}
	// Remove the host prefix if no more objects are left
	prefix := fmt.Sprintf("%s/", render.GetPrefixPath(wrender.CachedPagePrefix))
	empty, err := checkBucketPrefixEmpty(sqsClient, prefix)
	if err != nil {
		return err
	}
	if empty {
		if err := deletePrefixFromS3(sqsClient, prefix); err != nil {
			return err
		}
	}

	return nil
}

func (app *Application) DeleteDomainRenderCache(domain string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	sqsClient := s3.NewFromConfig(cfg)

	render, err := wrender.NewWrender(domain, wrender.CachedPagePrefix)
	if err != nil {
		return err
	}

	prefix := fmt.Sprintf("%s/", render.GetPrefixPath(wrender.CachedPagePrefix))
	if err := deletePrefixFromS3(sqsClient, prefix); err != nil {
		return err
	}

	return nil
}

type RenderStatusResp struct {
	Status  string   `json:"status"`
	Details []string `json:"details,omitempty"`
}

func (app *Application) CheckRenderStatus(key string) (RenderStatusResp, error) {
	var expirationInHours int
	var err error
	expirationConfig, exists := os.LookupEnv("JOB_EXPIRATION_IN_HOURS")
	if !exists {
		expirationInHours = 1
	} else {
		expirationInHours, err = strconv.Atoi(expirationConfig)
		if err != nil {
			return RenderStatusResp{}, err
		}
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return RenderStatusResp{}, err
	}
	client := s3.NewFromConfig(cfg)

	jobCache := wrender.NewSqsJobCache("", key, SitemapCategory)
	queueEmpty, err := checkBucketPrefixEmpty(client, jobCache.QueuedPath())
	if err != nil {
		return RenderStatusResp{}, err
	}
	processEmpty, err := checkBucketPrefixEmpty(client, jobCache.ProcessPath())
	if err != nil {
	}
	failureEmpty, err := checkBucketPrefixEmpty(client, jobCache.FailurePath())
	if err != nil {
		return RenderStatusResp{}, err
	}

	// Read job timestamp record
	now := time.Now().UTC()
	timestamp, err := readObjectFromS3(client, filepath.Join(jobCache.KeyPath(), timestampFile))
	if err != nil {
		return RenderStatusResp{}, err
	}
	parsedTime, err := time.Parse(time.RFC3339, string(timestamp))
	if err != nil {
		return RenderStatusResp{}, err
	}
	if now.Sub(parsedTime) > time.Duration(expirationInHours)*time.Hour {
		return RenderStatusResp{Status: jobStatusTimeout}, nil
	}

	if !queueEmpty || !processEmpty {
		return RenderStatusResp{Status: jobStatusProcessing}, nil
	} else if !failureEmpty {
		failureResp := RenderStatusResp{Status: jobStatusFailed, Details: []string{}}
		failureKeys, err := ListObjectsFromS3(client, jobCache.FailurePath())
		if err != nil {
			return RenderStatusResp{}, err
		}
		for _, key := range failureKeys {
			app.Logger.Debug(fmt.Sprintf("Failure key: %s", key))
			content, err := readObjectFromS3(client, key)
			if err != nil {
				return RenderStatusResp{}, err
			}

			var queuePayload workerQueuePayload
			if err := json.Unmarshal(content, &queuePayload); err != nil {
				return RenderStatusResp{}, err
			}
			failureResp.Details = append(failureResp.Details, queuePayload.TargetUrl)
		}

		return failureResp, nil
	}

	return RenderStatusResp{Status: jobStatusCompleted}, nil
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
