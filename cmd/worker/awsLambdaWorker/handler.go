package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/liuminhaw/wrenderer/internal/application/lambdaApp"
	"github.com/liuminhaw/wrenderer/wrender"
)

type handler struct {
	logger *slog.Logger
}

type workerQueuePayload struct {
	TargetUrl string `json:"targetUrl"`
	CacheKey  string `json:"cacheKey"`
}

func lambdaHandler(event events.SQSEvent) error {
	h := handler{}

	debugMode, exists := os.LookupEnv("WRENDERER_DEBUG_MODE")
	if !exists {
		h.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	} else if debugMode == "true" {
		h.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	} else {
		h.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	return h.sitemapHandler(event)
}

func (h *handler) sitemapHandler(event events.SQSEvent) error {
	for _, message := range event.Records {
		var payload workerQueuePayload
		if err := json.Unmarshal([]byte(message.Body), &payload); err != nil {
			h.logger.Error(
				"Failed to unmarshal message",
				slog.String("id", message.MessageId),
				slog.String("body", message.Body),
			)
			continue
		}

		h.logger.Debug(
			fmt.Sprintf("Processing url: %s", payload.TargetUrl),
			slog.String("cache key", payload.CacheKey),
			slog.String("id", message.MessageId),
		)

		app := &lambdaApp.Application{
			Logger: h.logger,
		}

		cfg, err := config.LoadDefaultConfig(context.Background())
		if err != nil {
			return h.workerError(message, err)
		}
		s3Client := s3.NewFromConfig(cfg)

		jobCache := wrender.NewSqsJobCache(
			message.MessageId,
			payload.CacheKey,
			lambdaApp.SitemapCategory,
		)

		// move job cache from queued to process
		if err := lambdaApp.UploadToS3(
			s3Client,
			jobCache.ProcessPath(),
			lambdaApp.PlainContentType,
			bytes.NewReader([]byte(message.Body)),
		); err != nil {
			return h.workerError(message, err)
		}
		if err := lambdaApp.DeleteObjectFromS3(s3Client, jobCache.QueuedPath()); err != nil {
			return h.workerError(message, err)
		}

		// render the target url
		_, err = app.RenderUrl(payload.TargetUrl, false)
		if err != nil {
			// Move job cache from process to failure
			if err := lambdaApp.UploadToS3(
				s3Client,
				jobCache.FailurePath(),
				lambdaApp.PlainContentType,
				bytes.NewReader([]byte(message.Body)),
			); err != nil {
				return h.workerError(message, err)
			}
			if err := lambdaApp.DeleteObjectFromS3(s3Client, jobCache.ProcessPath()); err != nil {
				return h.workerError(message, err)
			} else {
				h.logger.Debug(
					fmt.Sprintf("Job cache %s deleted", jobCache.ProcessPath()),
				)
			}

			return h.workerError(message, err)
		}

		// Clean job cache
		if err := lambdaApp.DeleteObjectFromS3(s3Client, jobCache.ProcessPath()); err != nil {
			return h.workerError(message, err)
		} else {
			h.logger.Debug(
				fmt.Sprintf("Job cache %s deleted", jobCache.ProcessPath()),
			)
		}

		h.logger.Debug(
			fmt.Sprintf("target url: %s processed", payload.TargetUrl),
			slog.String("cache key", payload.CacheKey),
			slog.String("id", message.MessageId),
		)
	}

	return nil
}
