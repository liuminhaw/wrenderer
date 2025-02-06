package awsLambda

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/internal/application/lambdaApp"
)

type handler struct {
	logger *slog.Logger
}

func LambdaHandler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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

	switch event.Path {
	case "/render":
		switch event.HTTPMethod {
		case "GET":
			handler.logger.Debug("request for rendering url")
			return handler.renderUrlHandler(event)
		case "PUT":
			handler.logger.Debug("request for rendering sitemap")
			return handler.renderSitemapHandler(event)
		case "DELETE":
			handler.logger.Debug("request for deleting rendered cache")
			return handler.deleteCacheHandler(event)
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

type renderResponse struct {
	Path string `json:"path"`
}

func (h *handler) renderUrlHandler(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	urlParam := event.QueryStringParameters["url"]
	h.logger.Info(fmt.Sprintf("Render url: %s", urlParam))
	if urlParam == "" {
		return h.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "Missing url parameter"},
		)
	}

	app := &lambdaApp.Application{
		Logger: h.logger,
	}
	cachePath, err := app.RenderUrl(urlParam)
	if err != nil {
		return h.serverError(event, err, nil)
	}

	responseBody, err := json.Marshal(renderResponse{Path: cachePath})
	if err != nil {
		return h.serverError(event, err, nil)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(responseBody),
	}, nil
}

func (h *handler) deleteCacheHandler(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	urlParam := event.QueryStringParameters["url"]
	h.logger.Debug("Delete cache", slog.String("url param", urlParam))

	domainParam := event.QueryStringParameters["domain"]
	h.logger.Debug("Delete cache", slog.String("domain param", domainParam))
	if urlParam == "" && domainParam == "" {
		return h.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "one of url or domain parameter is required"},
		)
	}

	app := &lambdaApp.Application{
		Logger: h.logger,
	}

	switch {
	case domainParam != "":
		h.logger.Info(fmt.Sprintf("Delete cache for domain: %s", domainParam))
		if err := app.DeleteDomainRenderCache(domainParam); err != nil {
			return h.serverError(event, err, nil)
		}
	case urlParam != "":
		h.logger.Info(fmt.Sprintf("Delete cache with url: %s", urlParam))
		if err := app.DeleteUrlRenderCache(urlParam); err != nil {
			return h.serverError(event, err, nil)
		}
	default:
		return h.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "one of url or domain parameter is required"},
		)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"message": "cache cleared"}`,
	}, nil
}

type renderSitemapPayload struct {
	SitemapUrl string `json:"sitemapUrl"`
}

func (h *handler) renderSitemapHandler(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	h.logger.Debug(
		fmt.Sprintf("Request body: %s", event.Body),
		slog.String("api", "renderSitemap"),
	)

	var payload renderSitemapPayload
	if err := json.Unmarshal([]byte(event.Body), &payload); err != nil {
		h.logger.Info(
			"Failed to unmarshal request body",
			slog.String("request body", event.Body),
		)
		return h.clientError(event, http.StatusBadRequest, nil)
	}

	if !internal.ValidUrl(payload.SitemapUrl) {
		h.logger.Info("Invalid sitemap url", slog.String("sitemap url", payload.SitemapUrl))
		return h.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "Invalid sitemap url"},
		)
	}

	app := &lambdaApp.Application{
		Logger: h.logger,
	}
	if err := app.RenderSitemap(payload.SitemapUrl); err != nil {
		return h.serverError(event, err, nil)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusAccepted,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Location":     "/current/placeholder",
		},
		Body: `{"message": "Sitemap rendering accepted"}`,
	}, nil
}
