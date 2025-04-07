package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/liuminhaw/wrenderer/cmd/shared"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/internal/application/lambdaApp"
	"github.com/liuminhaw/wrenderer/wrender"
)

type handler struct {
	logger *slog.Logger
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
	envConf, err := shared.LambdaReadEnv()
	if err != nil {
		h.logger.Error(fmt.Sprintf("Failed to read env: %v", err))
		return err
	}

	for _, message := range event.Records {
		var payload wrender.SqsJobPayload
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
			slog.String("cache key", payload.RandomKey),
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
			payload.RandomKey,
			internal.SitemapCategory,
			wrender.CachedJobPrefix,
		)
		// Tracing current caching state: start with queued
		caching := wrender.NewS3Caching(
			s3Client,
			jobCache.KeyPath(),
			filepath.Join(jobCache.KeyPath(), internal.JobStatusQueued, message.MessageId),
			wrender.S3CachingMeta{
				Bucket:      envConf.S3BucketName,
				Region:      envConf.S3BucketRegion,
				ContentType: wrender.PlainContentType,
			},
		)

		// move job cache from queued to process
		suffixPath := filepath.Join(internal.JobStatusProcessing, message.MessageId)
		if err := caching.UpdateTo(bytes.NewReader([]byte(message.Body)), suffixPath); err != nil {
			return h.workerError(message, err)
		}
		if err := caching.Delete(); err != nil {
			return h.workerError(message, err)
		}
		// caching state update to processing
		caching.CachedPath = filepath.Join(
			jobCache.KeyPath(),
			internal.JobStatusProcessing,
			message.MessageId,
		)

		// render the target url
		_, err = app.RenderUrl(payload.TargetUrl, false)
		if err != nil {
			// Move job cache from process to failure
			suffixPath := filepath.Join(internal.JobStatusFailed, message.MessageId)
			if err := caching.UpdateTo(bytes.NewReader([]byte(message.Body)), suffixPath); err != nil {
				return h.workerError(message, err)
			}
			if err := caching.Delete(); err != nil {
				return h.workerError(message, err)
			} else {
				h.logger.Debug(
					fmt.Sprintf("Job cache %s deleted", caching.CachedPath),
				)
			}

			return h.workerError(message, err)
		}

		// Clean job cache
		if err := caching.Delete(); err != nil {
			return h.workerError(message, err)
		} else {
			h.logger.Debug(
				fmt.Sprintf("Job cache %s deleted", caching.CachedPath),
			)
		}

		h.logger.Debug(
			fmt.Sprintf("target url: %s processed", payload.TargetUrl),
			slog.String("cache key", payload.RandomKey),
			slog.String("id", message.MessageId),
		)
	}

	return nil
}
