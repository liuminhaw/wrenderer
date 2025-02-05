package awsLambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"

	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/wrender"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type LambdaResponse struct {
	Path string `json:"path"`
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
			StatusCode: 400,
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

func (app *application) deleteRenderCache(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	urlParam := event.QueryStringParameters["url"]
	app.logger.Debug(fmt.Sprintf("Delete cache", slog.String("url param", urlParam)))

	domainParam := event.QueryStringParameters["domain"]
	app.logger.Debug(fmt.Sprintf("Delete cache", slog.String("domain param", domainParam)))
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

// uploadToS3 uploads the content to S3 bucket with given object key
func uploadToS3(client *s3.Client, objectKey string, content io.Reader) error {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return fmt.Errorf("uploadToS3: S3_BUCKET_NAME environment variable is not set")
	}
	if _, exists := os.LookupEnv("S3_BUCKET_REGION"); !exists {
		return fmt.Errorf("uploadToS3: S3_BUCKET_REGION environment variable is not set")
	}

	_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      &s3BucketName,
		Key:         &objectKey,
		Body:        content,
		ContentType: aws.String("text/html"),
	})
	if err != nil {
		return fmt.Errorf("uploadToS3: failed to upload object: %w", err)
	}

	return nil
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

// deleteObjectFromS3 deletes an object from S3 bucket with given object key
func deleteObjectFromS3(client *s3.Client, objectKey string) error {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return fmt.Errorf("deleteFromS3: S3_BUCKET_NAME environment variable is not set")
	}

	_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("deleteFromS3: failed to delete object: %w", err)
	}

	return nil
}

// deletePrefixFromS3 deletes all objects matching given prefix from S3 bucket
func deletePrefixFromS3(client *s3.Client, prefix string) error {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return fmt.Errorf("deletePrefixFromS3: S3_BUCKET_NAME environment variable is not set")
	}

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s3BucketName),
		Prefix: aws.String(prefix),
	}

	for {
		result, err := client.ListObjectsV2(context.Background(), input)
		if err != nil {
			return fmt.Errorf("deletePrefixFromS3: list objects: %w", err)
		}

		if len(result.Contents) == 0 {
			// No object left
			break
		}

		// Prepare the objects to delete
		var objectsToDelete []types.ObjectIdentifier
		for _, object := range result.Contents {
			objectsToDelete = append(objectsToDelete, types.ObjectIdentifier{
				Key: object.Key,
			})
		}

		// Perform the delete operation
		_, err = client.DeleteObjects(context.Background(), &s3.DeleteObjectsInput{
			Bucket: aws.String(s3BucketName),
			Delete: &types.Delete{
				Objects: objectsToDelete,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return fmt.Errorf("deletePrefixFromS3: delete objects: %w", err)
		}

		// Check if there are more objects to delete
		if *result.IsTruncated {
			input.ContinuationToken = result.NextContinuationToken
		} else {
			break
		}
	}

	return nil
}

func (app *application) renderPage(urlParam string) ([]byte, error) {
	idleType, exists := os.LookupEnv("WRENDERER_IDLE_TYPE")
	if !exists {
		idleType = "networkIdle"
	}

	var windowWidth, windowHeight int
	var err error
	windowWidthConfig, exists := os.LookupEnv("WRENDERER_WINDOW_WIDTH")
	if !exists {
		windowWidth = 1920
	} else {
		windowWidth, err = strconv.Atoi(windowWidthConfig)
		if err != nil {
			return nil, fmt.Errorf("renderPage: %w", err)
		}
	}
	windowHeightConfig, exists := os.LookupEnv("WRENDERER_WINDOW_HEIGHT")
	if !exists {
		windowHeight = 1080
	} else {
		windowHeight, err = strconv.Atoi(windowHeightConfig)
		if err != nil {
			return nil, fmt.Errorf("renderPage: %w", err)
		}
	}

	userAgent, exists := os.LookupEnv("WRENDERER_USER_AGENT")
	if !exists {
		userAgent = ""
	}

	r := renderer.NewRenderer(renderer.WithLogger(app.logger))
	content, err := r.RenderPage(urlParam, &renderer.RendererOption{
		BrowserOpts: renderer.BrowserConf{
			IdleType:  idleType,
			Container: true,
		},
		Headless:     true,
		WindowWidth:  windowWidth,
		WindowHeight: windowHeight,
		Timeout:      30,
		UserAgent:    userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("renderPage: %w", err)
	}

	return content, nil
}
