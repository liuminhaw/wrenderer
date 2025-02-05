package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

type application struct {
	logger *slog.Logger
}

type workerQueuePayload struct {
	TargetUrl string `json:"targetUrl"`
}

func lambdaHandler(event events.SQSEvent) error {
	var logger *slog.Logger

	debugMode, exists := os.LookupEnv("WRENDERER_DEBUG_MODE")
	if !exists {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	} else if debugMode == "true" {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	for _, message := range event.Records {
		var payload workerQueuePayload
		if err := json.Unmarshal([]byte(message.Body), &payload); err != nil {
			logger.Error(
				"Failed to unmarshal message",
				slog.String("id", message.MessageId),
				slog.String("body", message.Body),
			)
			continue
		}

		logger.Debug(
			fmt.Sprintf("Target Url: %s", payload.TargetUrl),
			slog.String("id", message.MessageId),
		)
	}

	return nil
}
