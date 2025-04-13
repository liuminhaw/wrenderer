package awsLambda

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/liuminhaw/wrenderer/cmd/shared"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
)

func deleteDomainRenderCache(domain string) error {
	loader, err := shared.NewConfLoader(shared.S3Service)
	if err != nil {
		return err
	}

	render, err := wrender.NewWrender(domain, wrender.CachedPagePrefix)
	if err != nil {
		return err
	}
	caching := wrender.NewS3Caching(
		loader.Clients.S3,
		render.GetPrefixPath(),
		render.CachePath,
		wrender.S3CachingMeta{
			Bucket:      loader.EnvConf.S3BucketName,
			Region:      loader.EnvConf.S3BucketRegion,
			ContentType: wrender.HtmlContentType,
		},
	)

	return caching.DeletePrefix()
}

func deleteUrlRenderCache(url string) error {
	loader, err := shared.NewConfLoader(shared.S3Service)
	if err != nil {
		return err
	}

	render, err := wrender.NewWrender(url, wrender.CachedPagePrefix)
	if err != nil {
		return err
	}
	caching := wrender.NewS3Caching(
		loader.Clients.S3,
		render.GetPrefixPath(),
		render.CachePath,
		wrender.S3CachingMeta{
			Bucket:      loader.EnvConf.S3BucketName,
			Region:      loader.EnvConf.S3BucketRegion,
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

func renderSitemap(url string, logger *slog.Logger) (string, error) {
	loader, err := shared.NewConfLoader(shared.S3Service, shared.SqsService)
	if err != nil {
		return "", err
	}

	entries, err := internal.ParseSitemap(url)
	if err != nil {
		return "", err
	}

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
		loader.Clients.S3,
		jobCache.KeyPath(),
		"",
		wrender.S3CachingMeta{
			Bucket:      loader.EnvConf.S3BucketName,
			Region:      loader.EnvConf.S3BucketRegion,
			ContentType: wrender.PlainContentType,
		},
	)

	now := time.Now().UTC().Format(time.RFC3339)
	if err := caching.UpdateTo(bytes.NewReader([]byte(now)), timestampFile); err != nil {
		return "", err
	}

	queue := shared.Queue{
		Client: loader.Clients.Sqs,
		Url:    loader.EnvConf.SqsUrl,
	}
	for _, entry := range entries {
		logger.Debug(fmt.Sprintf("Entry: %s", entry.Loc))
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
		logger.Debug(
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

func checkRenderStatus(key string, logger *slog.Logger) (shared.RenderStatusResp, error) {
	loader, err := shared.NewConfLoader(shared.S3Service)
	if err != nil {
		return shared.RenderStatusResp{}, err
	}

	jobCache := wrender.NewSqsJobCache(key, internal.SitemapCategory, wrender.CachedJobPrefix)
	caching := wrender.NewS3Caching(
		loader.Clients.S3,
		jobCache.KeyPath(),
		filepath.Join(jobCache.KeyPath(), timestampFile),
		wrender.S3CachingMeta{
			Bucket:      loader.EnvConf.S3BucketName,
			Region:      loader.EnvConf.S3BucketRegion,
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
	if now.Sub(parsedTime) > time.Duration(loader.EnvConf.JobExpirationInHours)*time.Hour {
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
			logger.Debug(fmt.Sprintf("Failure object key: %s", content.Path))

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
