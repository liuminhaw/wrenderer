package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/liuminhaw/wrenderer/internal/application/lambdaApp"
)

type handler struct {
	logger *slog.Logger
}

type workerQueuePayload struct {
	TargetUrl string `json:"targetUrl"`
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
			slog.String("id", message.MessageId),
		)

		app := &lambdaApp.Application{
			Logger: h.logger,
		}

		// delete existing cache
		if err := app.DeleteUrlRenderCache(payload.TargetUrl); err != nil {
			h.logger.Error(
				err.Error(),
				slog.String("id", message.MessageId),
				slog.Any("payload", payload),
			)
			return err
		}

		// render the target url
		_, err := app.RenderUrl(payload.TargetUrl)
		if err != nil {
			h.logger.Error(
				err.Error(),
				slog.String("id", message.MessageId),
				slog.Any("payload", payload),
			)
			return err
		}

		h.logger.Debug(
			fmt.Sprintf("target url: %s processed", payload.TargetUrl),
			slog.String("id", message.MessageId),
		)
	}

	return nil
}
