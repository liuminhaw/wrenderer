package awsLambda

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/liuminhaw/wrenderer/cmd/shared"
	"github.com/liuminhaw/wrenderer/cmd/shared/lambdaApp"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"
)

const (
	JobKeyLength  = 6
	timestampFile = "timestamp"
)

type handler struct {
	logger *slog.Logger
}

type renderResponse struct {
	Path string `json:"path"`
}

func (h *handler) getRenderHandleFunc(
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

	cachePath, err := lambdaApp.RenderUrl(urlParam, true, h.logger)
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

func (h *handler) deleteRenderHandleFunc(
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

	switch {
	case domainParam != "":
		h.logger.Info(fmt.Sprintf("Delete cache for domain: %s", domainParam))
		if err := deleteDomainRenderCache(domainParam); err != nil {
			return h.serverError(event, err, nil)
		}
	case urlParam != "":
		h.logger.Info(fmt.Sprintf("Delete cache with url: %s", urlParam))
		if err := deleteUrlRenderCache(urlParam); err != nil {
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

func (h *handler) putRenderSitemapHandleFunc(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	h.logger.Debug(
		fmt.Sprintf("Request body: %s", event.Body),
		slog.String("api", "renderSitemap"),
	)

	var payload shared.RenderSitemapPayload
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

	location, err := renderSitemap(payload.SitemapUrl, h.logger)
	if err != nil {
		return h.serverError(event, err, nil)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusAccepted,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Location":     fmt.Sprintf("/render/sitemap/%s/status", location),
		},
		Body: fmt.Sprintf(
			"{\"message\": \"Sitemap rendering accepted\", \"location\": \"/render/sitemap/%s/status\"}",
			location,
		),
	}, nil
}

type jobStatusResponse struct {
	Status  string   `json:"status"`
	Details []string `json:"details,omitempty"`
}

func (h *handler) getRenderSitemapStatusHandleFunc(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	jobId := event.PathParameters["id"]
	h.logger.Debug("Check job status", slog.String("jobId", jobId))

	statusResp, err := checkRenderStatus(jobId, h.logger)
	if err != nil {
		var werr *wrender.CacheNotFoundError
		if errors.As(err, &werr) {
			h.logger.Info(
				"Status of sitemap job not found",
				slog.String("job id", jobId),
				slog.String("error", err.Error()),
			)
			return h.clientError(
				event,
				http.StatusNotFound,
				&respErrorMessage{Message: "status not found"},
			)
		}
		return h.serverError(event, err, nil)
	}

	responseBody, err := json.Marshal(statusResp)
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
