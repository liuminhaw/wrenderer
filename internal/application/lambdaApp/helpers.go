package lambdaApp

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

type respErrorMessage struct {
	Message string `json:"message"`
}

func (app *Application) serverError(
	event events.APIGatewayProxyRequest,
	cause error,
	message *respErrorMessage,
) (events.APIGatewayProxyResponse, error) {
	var (
		method      = event.HTTPMethod
		path        = event.Path
		queryString = event.QueryStringParameters
		body        = event.Body
	)

	if message == nil {
		message = &respErrorMessage{Message: "Internal server error"}
	}

	var respBody string
	respMsg, err := json.Marshal(*message)
	if err != nil {
		app.Logger.Error(
			"serverError failed to marshal response message",
			slog.Any("message", message),
		)
		respBody = `{"message":"Internal server error"}`
	} else {
		app.Logger.Error(
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

func (app *Application) clientError(
	event events.APIGatewayProxyRequest,
	status int,
	message *respErrorMessage,
) (events.APIGatewayProxyResponse, error) {
	var (
		method      = event.HTTPMethod
		path        = event.Path
		queryString = event.QueryStringParameters
		body        = event.Body
	)

	if message == nil {
		message = &respErrorMessage{Message: "Client error"}
	}
	var respBody string
	respMsg, err := json.Marshal(*message)
	if err != nil {
		app.Logger.Error(
			"serverError failed to marshal response message",
			slog.Any("message", message),
		)
		respBody = `{"message":"Internal server error"}`
	} else {
		app.Logger.Error(
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
