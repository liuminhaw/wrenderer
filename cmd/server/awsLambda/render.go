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
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Missing url parameter",
		}, nil
	}

	render, err := wrender.NewWrender(urlParam)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "New wrender error",
		}, nil
	}

	// Check if object exists
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: fmt.Sprintf("Failed to load config: %s", err),
		}, nil
	}
	client := s3.NewFromConfig(cfg)

	exists, err := checkObjectExists(client, render.CachePath)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: fmt.Sprintf("Failed to check object exists: %s", err),
		}, nil
	}
	if exists {
		responseBody, err := json.Marshal(LambdaResponse{Path: render.CachePath})
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: fmt.Sprintf("Failed to marshal response body: %s", err),
			}, nil
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
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Render Failed",
		}, nil
	}

	// Check if rendered result is empty
	if len(content) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Rendered failed with empty content",
		}, nil
	}

	// Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	err = uploadToS3(client, render.CachePath, contentReader)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: fmt.Sprintf("Failed to upload to S3: %s", err),
		}, err
	}
	// TODO: Return S3 URL for modify request settings
	responseBody, err := json.Marshal(LambdaResponse{Path: render.CachePath})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: fmt.Sprintf("Failed to marshal response body: %s", err),
		}, nil
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
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Bad request",
		}, nil
	}

	if !internal.ValidUrl(payload.SitemapUrl) {
		app.logger.Info("Invalid sitemap url", slog.String("sitemap url", payload.SitemapUrl))
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Invalid sitemap url",
		}, nil
	}

	resp, err := http.Get(payload.SitemapUrl)
	if err != nil {
		app.logger.Error(
			fmt.Sprintf("Failed to fetch sitemap: %s", payload.SitemapUrl),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Internal Server Error",
		}, nil
	}

	entries, err := sitemapHelper.ParseSitemap(resp.Body)
	if err != nil {
		app.logger.Error(
			fmt.Sprintf("Failed to parse sitemap from %s", payload.SitemapUrl),
		)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Internal Server Error",
		}, nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		app.logger.Error("Failed to load aws sdk config")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Internal Server Error",
		}, nil
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
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "Internal Server Error",
			}, nil
		}

		messageId, err := sendMessageToQueue(client, string(payload))
		if err != nil {
			app.logger.Error(
				"Failed to send message to worker queue",
				slog.String("url", entry.Loc),
			)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: "Internal Server Error",
			}, nil
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
		Body: "{\"message\": \"Sitemap rendering accepted\"}",
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
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "one of url or domain parameter is required",
		}, nil
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: fmt.Sprintf("Failed to load config: %s", err),
		}, nil
	}
	client := s3.NewFromConfig(cfg)

	switch {
	case domainParam != "":
		app.logger.Info(fmt.Sprintf("Delete cache for domain: %s", domainParam))
		if err := clearDomainCache(client, domainParam); err != nil {
			app.logger.Error(err.Error(), slog.String("domain param", domainParam))
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: fmt.Sprintf("Error clearing domain from cache: %s", domainParam),
			}, nil
		}
	case urlParam != "":
		app.logger.Info(fmt.Sprintf("Delete cache of url: %s", urlParam))
		if err := clearUrlCache(client, urlParam); err != nil {
			app.logger.Error(err.Error(), slog.String("url param", urlParam))
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: fmt.Sprintf("Error clearing url from cache: %s", urlParam),
			}, nil
		}
	default:
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "one of url or domain parameter is required",
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "text/plain",
		},
		Body: "Cache cleared",
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
