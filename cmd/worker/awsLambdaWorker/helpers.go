package main

import (
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
)

func (h *handler) workerError(event events.SQSMessage, err error) error {
	var (
		messageId   = event.MessageId
		messageBody = event.Body
	)

	h.logger.Error(err.Error(), slog.String("id", messageId), slog.String("body", messageBody))
	return err
}
