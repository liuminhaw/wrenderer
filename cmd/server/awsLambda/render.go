package awsLambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/liuminhaw/renderer"
	"github.com/liuminhaw/wrenderer/shared"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// renderUrl is the handler for rendering given url from query parameters
// It returns the rendered object key in S3 bucket along with the host and port
func renderUrl(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get query parameters
	urlParam := event.QueryStringParameters["url"]
	log.Printf("Render url: %s", urlParam)
	if urlParam == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Missing url parameter",
		}, nil
	}

	urlDest, err := url.Parse(urlParam)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Invalid url parameter",
		}, nil
	}

	objectKey, err := calcKey([]byte(urlParam))
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: fmt.Sprintf("Failed to calculate object key: %s", err),
		}, nil
	}
	responseBodyData := shared.ResponseData{
		Host:      urlDest.Hostname(),
		Port:      urlDest.Port(),
		ObjectKey: objectKey,
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

	objectPathKey := responseBodyData.GetObjectPath()

	exists, err := checkObjectExists(client, objectPathKey)
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
		responseBody, err := json.Marshal(responseBodyData)
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
	content, err := renderPage(urlParam)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/plain",
			},
			Body: "Render Failed",
		}, nil
	}

	// TODO: Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	err = uploadToS3(client, objectPathKey, contentReader)
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
	responseBody, err := json.Marshal(responseBodyData)
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

func deleteRenderCache(
	event events.APIGatewayProxyRequest,
) (events.APIGatewayProxyResponse, error) {
	urlParam := event.QueryStringParameters["url"]
	log.Printf("Delete render cache for url: %s", urlParam)

	domainParam := event.QueryStringParameters["domain"]
	log.Printf("Delete render cache for domain: %s", domainParam)
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
		log.Printf("Delete render cache for domain: %s", domainParam)
		if err := clearDomainCache(client, domainParam); err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Headers: map[string]string{
					"Content-Type": "text/plain",
				},
				Body: fmt.Sprintf("Error clearing domain from cache: %s", domainParam),
			}, nil

		}
	case urlParam != "":
		log.Printf("Delete render cache for url: %s", urlParam)
		if err := clearUrlCache(client, urlParam); err != nil {
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
		Bucket: &s3BucketName,
		Key:    &objectKey,
		Body:   content,
		ACL:    types.ObjectCannedACLPublicRead,
	})
	if err != nil {
		return fmt.Errorf("uploadToS3: failed to upload object: %w", err)
	}

	return nil
}

func clearDomainCache(client *s3.Client, domain string) error {
    parsedUrl, err := url.Parse(domain)
    if err != nil {
        return fmt.Errorf("clearDomainCache: failed to parse domain: %w", err)
    }

	prefix := fmt.Sprintf("%s/", parsedUrl.Hostname())
	if err := deletePrefixFromS3(client, prefix); err != nil {
		return fmt.Errorf("clearDomainCache: delete domain cache: %w", err)
	}

	return nil
}

func clearUrlCache(client *s3.Client, urlParam string) error {
	parsedUrl, err := url.Parse(urlParam)
	if err != nil {
		return fmt.Errorf("clearUrlCache: failed to parse url: %w", err)
	}

	objectKey, err := calcKey([]byte(urlParam))
	if err != nil {
		return fmt.Errorf("clearUrlCache: calculate object key: %w", err)
	}

	objectData := shared.ResponseData{
		Host:      parsedUrl.Hostname(),
		Port:      parsedUrl.Port(),
		ObjectKey: objectKey,
	}
	objectPathKey := objectData.GetObjectPath()

	// Remove the object from s3
	if err := deleteObjectFromS3(client, objectPathKey); err != nil {
		return fmt.Errorf("clearUrlCache: delete object: %w", err)
	}
	// Remove the host prefix if no more objects are left
	empty, err := checkDomainEmpty(client, objectData.Host)
	if err != nil {
		return fmt.Errorf("clearUrlCache: check empty domain cache: %w", err)
	}
	if empty {
		prefix := fmt.Sprintf("%s/", objectData.Host)
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

func renderPage(urlParam string) ([]byte, error) {
	browserContext := renderer.BrowserContext{
		DebugMode: true,
		Container: true,
		// SingleProcess:   true,
	}
	rendererContext := renderer.RendererContext{
		Headless:       true,
		WindowWidth:    1000,
		WindowHeight:   1000,
		Timeout:        30,
		ImageLoad:      false,
		SkipFrameCount: 0,
	}
	ctx := context.Background()
	ctx = renderer.WithBrowserContext(ctx, &browserContext)
	ctx = renderer.WithRendererContext(ctx, &rendererContext)

	content, err := renderer.RenderPage(ctx, urlParam)
	if err != nil {
		return nil, fmt.Errorf("renderPage: %w", err)
	}

	// Regular expressions for matching base64 images and SVG content
	// regexpBase64 := regexp.MustCompile(`"data:image\/.*?;base64.*?"`)
	// regexpSVG := regexp.MustCompile(`\<svg.*?\>.*?\<\/svg\>`)

	// Replacing base64 images with empty strings and SVG content with empty <svg></svg> tags
	// newContext := regexpBase64.ReplaceAllString(string(content), `""`)
	// newContext = regexpSVG.ReplaceAllString(newContext, `<svg></svg>`)

	return content, nil
}
