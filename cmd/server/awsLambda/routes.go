package awsLambda

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"github.com/aws/aws-lambda-go/events"
)

func Routes(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	handler := handler{}

	debugMode, exists := os.LookupEnv("WRENDERER_DEBUG_MODE")
	if !exists {
		handler.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	} else if debugMode == "true" {
		handler.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		}))
	} else {
		handler.logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

	handler.logger.Info(fmt.Sprintf("Request path: %s", event.Path))
	handler.logger.Info(fmt.Sprintf("HTTP method: %s", event.HTTPMethod))

	jobStatusPattern := regexp.MustCompile(
		fmt.Sprintf("^/render/sitemap/[a-zA-Z]{6}-[a-zA-Z]{6}/status$"),
	)
	switch {
	case "/render" == event.Path:
		switch event.HTTPMethod {
		case "GET":
			handler.logger.Debug("request for rendering url")
			return handler.getRenderHandleFunc(event)
		case "DELETE":
			handler.logger.Debug("request for deleting rendered cache")
			return handler.deleteRenderHandleFunc(event)
		default:
			return events.APIGatewayProxyResponse{
				StatusCode: 405,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "Method Not Allowed",
			}, nil
		}
	case "/render/sitemap" == event.Path:
		switch event.HTTPMethod {
		case "PUT":
			handler.logger.Debug("request for rendering sitemap")
			return handler.putRenderSitemapHandleFunc(event)
		default:
			return events.APIGatewayProxyResponse{
				StatusCode: 405,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "Method Not Allowed",
			}, nil
		}
	case jobStatusPattern.MatchString(event.Path):
		switch event.HTTPMethod {
		case "GET":
			handler.logger.Debug(
				"request for checking job status",
				slog.String("id", event.PathParameters["id"]),
			)
			return handler.getRenderSitemapStatusHandleFunc(event)
		default:
			return events.APIGatewayProxyResponse{
				StatusCode: 405,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "Method Not Allowed",
			}, nil
		}
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Not Found",
		}, nil
	}
}
