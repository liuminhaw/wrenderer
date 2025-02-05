package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
)

type application struct {
	logger *slog.Logger
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

	logger.Debug("Processing event")
	for _, message := range event.Records {
		logger.Debug(
			"Processing message",
			slog.String("id", message.MessageId),
			slog.String("body", message.Body),
		)
	}

	return nil
}
