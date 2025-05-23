package awsLambda

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/liuminhaw/wrenderer/cmd/shared"
)

func (h *handler) serverError(
	event events.APIGatewayProxyRequest,
	cause error,
	message *shared.RespErrorMessage,
) (events.APIGatewayProxyResponse, error) {
	var (
		method      = event.HTTPMethod
		path        = event.Path
		queryString = event.QueryStringParameters
		body        = event.Body
	)

	if message == nil {
		message = &shared.RespErrorMessage{Message: "Internal server error"}
	}

	var respBody string
	respMsg, err := json.Marshal(*message)
	if err != nil {
		h.logger.Error(
			"serverError failed to marshal response message",
			slog.Any("message", message),
		)
		respBody = `{"message":"Internal server error"}`
	} else {
		h.logger.Error(
			cause.Error(),
			slog.String("method", method),
			slog.String("path", path),
			slog.Any("queryString", queryString),
			slog.String("body", body),
		)
		respBody = string(respMsg)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: respBody,
	}, nil
}

func (h *handler) clientError(
	event events.APIGatewayProxyRequest,
	status int,
	message *shared.RespErrorMessage,
) (events.APIGatewayProxyResponse, error) {
	var (
		method      = event.HTTPMethod
		path        = event.Path
		queryString = event.QueryStringParameters
		body        = event.Body
	)

	if message == nil {
		message = &shared.RespErrorMessage{Message: "Client error"}
	}
	var respBody string
	respMsg, err := json.Marshal(*message)
	if err != nil {
		h.logger.Error(
			"serverError failed to marshal response message",
			slog.Any("message", message),
		)
		respBody = `{"message":"Internal server error"}`
	} else {
		h.logger.Info(
			"client error",
			slog.String("method", method),
			slog.String("path", path),
			slog.Any("queryString", queryString),
			slog.String("body", body),
		)
		respBody = string(respMsg)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: respBody,
	}, nil
}
