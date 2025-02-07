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
			return err
		}
		s3Client := s3.NewFromConfig(cfg)

		jobCache := wrender.NewSqsJobCache(
			message.MessageId,
			payload.CacheKey,
			lambdaApp.SitemapCategory,
		)

		// render the target url
		_, err = app.RenderUrl(payload.TargetUrl, false)
		if err != nil {
			h.logger.Error(
				err.Error(),
				slog.String("id", message.MessageId),
				slog.Any("payload", payload),
			)

			// Create job cache in failure path
			if err := lambdaApp.UploadToS3(
				s3Client,
				jobCache.FailurePath(),
				lambdaApp.PlainContentType,
				bytes.NewReader([]byte(message.Body)),
			); err != nil {
				h.logger.Error(
					err.Error(),
					slog.String("id", message.MessageId),
					slog.Any("payload", payload),
				)

				return err
			}

            // Remove job from process path in cache
			if err := lambdaApp.DeleteObjectFromS3(s3Client, jobCache.ProcessPath()); err != nil {
				h.logger.Error(
					err.Error(),
					slog.String("id", message.MessageId),
					slog.Any("payload", payload),
				)
				return err
			} else {
				h.logger.Debug(
					fmt.Sprintf("Job cache %s deleted", jobCache.ProcessPath()),
				)
			}

			return err
		}

		// Clean job cache
		if err := lambdaApp.DeleteObjectFromS3(s3Client, jobCache.ProcessPath()); err != nil {
			h.logger.Error(
				err.Error(),
				slog.String("id", message.MessageId),
				slog.Any("payload", payload),
			)
			return err
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
