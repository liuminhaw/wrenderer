package awsLambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/liuminhaw/sitemapHelper"
	"github.com/liuminhaw/wrenderer/internal"
	"github.com/liuminhaw/wrenderer/wrender"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type LambdaResponse struct {
	Path string `json:"path"`
}

type renderSitemapPayload struct {
	SitemapUrl string `json:"sitemapUrl"`
}

// renderUrl is the handler for rendering given url from query parameters
// It returns the rendered object key in S3 bucket along with the host and port
func (app *application) renderUrl(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	// Get query parameters
	urlParam := event.QueryStringParameters["url"]
	app.logger.Info(fmt.Sprintf("Render url: %s", urlParam))
	if urlParam == "" {
		return app.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "Missing url parameter"},
		)
	}

	render, err := wrender.NewWrender(urlParam)
	if err != nil {
		return app.serverError(event, err, nil)
	}

	// Check if object exists
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return app.serverError(event, err, nil)
	}
	client := s3.NewFromConfig(cfg)

	exists, err := checkObjectExists(client, render.CachePath)
	if err != nil {
		return app.serverError(event, err, nil)
	}
	if exists {
		responseBody, err := json.Marshal(LambdaResponse{Path: render.CachePath})
		if err != nil {
			return app.serverError(event, err, nil)
		}

		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: string(responseBody),
		}, nil
	}

	// Render the page
	content, err := app.renderPage(urlParam)
	if err != nil {
		return app.serverError(event, err, nil)
	}

	// Check if rendered result is empty
	if len(content) == 0 {
		return app.serverError(
			event,
			err,
			&respErrorMessage{Message: "Rendered failed with empty content"},
		)
	}

	// Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	err = uploadToS3(client, render.CachePath, contentReader)
	if err != nil {
		return app.serverError(event, err, nil)
	}

	// TODO: Return S3 URL for modify request settings
	responseBody, err := json.Marshal(LambdaResponse{Path: render.CachePath})
	if err != nil {
		return app.serverError(event, err, nil)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(responseBody),
	}, nil
}

type workerQueuePayload struct {
	TargetUrl string `json:"targetUrl"`
}

func (app *application) renderSitemap(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	app.logger.Debug(
		fmt.Sprintf("Request body: %s", event.Body),
		slog.String("api", "renderSitemap"),
	)

	var payload renderSitemapPayload
	if err := json.Unmarshal([]byte(event.Body), &payload); err != nil {
		app.logger.Info(
			"Failed to unmarshal request body",
			slog.String("request body", event.Body),
		)
		return app.clientError(event, http.StatusBadRequest, nil)
	}

	if !internal.ValidUrl(payload.SitemapUrl) {
		app.logger.Info("Invalid sitemap url", slog.String("sitemap url", payload.SitemapUrl))
		return app.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "Invalid sitemap url"},
		)
	}

	resp, err := http.Get(payload.SitemapUrl)
	if err != nil {
		app.logger.Error(
			fmt.Sprintf("Failed to fetch sitemap: %s", payload.SitemapUrl),
		)
		return app.serverError(event, err, nil)
	}

	entries, err := sitemapHelper.ParseSitemap(resp.Body)
	if err != nil {
		app.logger.Error(
			fmt.Sprintf("Failed to parse sitemap from %s", payload.SitemapUrl),
		)
		return app.serverError(event, err, nil)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		app.logger.Error("Failed to load aws sdk config")
		return app.serverError(event, err, nil)
	}
	client := sqs.NewFromConfig(cfg)

	for _, entry := range entries {
		app.logger.Debug(fmt.Sprintf("Entry: %s", entry.Loc))
		payload, err := json.Marshal(workerQueuePayload{TargetUrl: entry.Loc})
		if err != nil {
			app.logger.Error(
				"Failed to generate worker queue payload",
				slog.String("url", entry.Loc),
			)
			return app.serverError(event, err, nil)
		}

		messageId, err := sendMessageToQueue(client, string(payload))
		if err != nil {
			app.logger.Error(
				"Failed to send message to worker queue",
				slog.String("url", entry.Loc),
			)
			return app.serverError(event, err, nil)
		}
		app.logger.Debug(
			fmt.Sprintf("Message id %s successfully sent", messageId),
			slog.String("payload", string(payload)),
		)
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

func (app *application) deleteRenderCache(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	urlParam := event.QueryStringParameters["url"]
	app.logger.Debug("Delete cache", slog.String("url param", urlParam))

	domainParam := event.QueryStringParameters["domain"]
	app.logger.Debug("Delete cache", slog.String("domain param", domainParam))
	if urlParam == "" && domainParam == "" {
		return app.clientError(
			event,
			http.StatusBadRequest,
			&respErrorMessage{Message: "one of url or domain parameter is required"},
		)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return app.serverError(event, err, nil)
	}
	client := s3.NewFromConfig(cfg)

	switch {
	case domainParam != "":
		app.logger.Info(fmt.Sprintf("Delete cache for domain: %s", domainParam))
		if err := clearDomainCache(client, domainParam); err != nil {
			app.logger.Error(err.Error(), slog.String("domain param", domainParam))
			return app.serverError(event, err, nil)
		}
	case urlParam != "":
		app.logger.Info(fmt.Sprintf("Delete cache of url: %s", urlParam))
		if err := clearUrlCache(client, urlParam); err != nil {
			app.logger.Error(err.Error(), slog.String("url param", urlParam))
			return app.serverError(event, err, nil)
		}
	default:
		return app.clientError(
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

func clearDomainCache(client *s3.Client, domain string) error {
	render, err := wrender.NewWrender(domain)
	if err != nil {
		return fmt.Errorf("clearDomainCache: %w", err)
	}

	prefix := fmt.Sprintf("%s/", render.GetPrefixPath())
	if err := deletePrefixFromS3(client, prefix); err != nil {
		return fmt.Errorf("clearDomainCache: delete domain cache: %w", err)
	}

	return nil
}

func clearUrlCache(client *s3.Client, urlParam string) error {
	render, err := wrender.NewWrender(urlParam)
	if err != nil {
		return fmt.Errorf("clearUrlCache: new wrender: %w", err)
	}

	// Remove the object from s3
	if err := deleteObjectFromS3(client, render.CachePath); err != nil {
		return fmt.Errorf("clearUrlCache: delete object: %w", err)
	}
	// Remove the host prefix if no more objects are left
	prefix := fmt.Sprintf("%s/", render.GetPrefixPath())
	empty, err := checkDomainEmpty(client, prefix)
	if err != nil {
		return fmt.Errorf("clearUrlCache: check empty domain cache: %w", err)
	}
	if empty {
		if err := deletePrefixFromS3(client, prefix); err != nil {
			return fmt.Errorf("clearUrlCache: delete domain cache: %w", err)
		}
	}

	return nil
}
