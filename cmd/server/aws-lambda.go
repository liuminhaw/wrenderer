package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/liuminhaw/renderer"
)

type responseData struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	ObjectKey string `json:"objectKey"`
}

func (rd responseData) getObjectPath() string {
	var objectPath string
	if rd.Port != "" {
		objectPath = strings.Join([]string{rd.Host, rd.Port}, "_")
		objectPath = strings.Join([]string{objectPath, rd.ObjectKey}, "/")
	} else {
		objectPath = strings.Join([]string{rd.Host, rd.ObjectKey}, "/")
	}

	return objectPath
}

func LambdaHandler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get query parameters
	urlParam := event.QueryStringParameters["url"]
	log.Printf("url: %s", urlParam)
	if urlParam == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Missing url parameter",
		}, nil
	}

	urlDest, err := url.Parse(urlParam)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "Invalid url parameter",
		}, nil
	}

	objectKey, err := calcKey([]byte(urlParam))
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to calculate object key: %s", err),
		}, nil
	}
	responseBodyData := responseData{
		Host:      urlDest.Hostname(),
		Port:      urlDest.Port(),
		ObjectKey: objectKey,
	}

	// Check if object exists
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to load config: %s", err),
		}, nil
	}
	client := s3.NewFromConfig(cfg)

	objectPathKey := responseBodyData.getObjectPath()

	exists, err := checkObjectExists(client, objectPathKey)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to check object exists: %s", err),
		}, nil
	}
	if exists {
		responseBody, err := json.Marshal(responseBodyData)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to marshal response body: %s", err),
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
			Body:       "Render Failed",
		}, nil
	}

	// TODO: Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	err = uploadToS3(objectPathKey, contentReader)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to upload to S3: %s", err),
		}, err
	}
	// TODO: Return S3 URL for modify request settings
	responseBody, err := json.Marshal(responseBodyData)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       fmt.Sprintf("Failed to marshal response body: %s", err),
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

// checkObjectExists checks if an object exists in S3 bucket or if the object is empty
// Returns true if the object exists and false if it does not exist
// Error is returned if there is an error checking the object
func checkObjectExists(client *s3.Client, objectKey string) (bool, error) {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return false, fmt.Errorf(
			"checkObjectExists: S3_BUCKET_NAME environment variable is not set",
		)
	}

	objStats, err := client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		var apiErr smithy.APIError
		if ok := errors.As(err, &apiErr); ok {
			switch apiErr.ErrorCode() {
			case "NotFound":
				return false, nil
			default:
				return false, fmt.Errorf("checkObjectExists: %w", err)
			}
		} else {
			return false, fmt.Errorf("checkObjectExists: %w", err)
		}
	}

	// Check object content length
	if *objStats.ContentLength == 0 {
		return false, nil
	}

	// Object exists
	return true, nil
}

func uploadToS3(objectKey string, content io.Reader) error {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return fmt.Errorf("uploadToS3: S3_BUCKET_NAME environment variable is not set")
	}
	if _, exists := os.LookupEnv("S3_BUCKET_REGION"); !exists {
		return fmt.Errorf("uploadToS3: S3_BUCKET_REGION environment variable is not set")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("uploadToS3: failed to load config: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
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

func calcKey(input []byte) (string, error) {
	h := sha256.New()
	_, err := h.Write(input)
	if err != nil {
		return "", fmt.Errorf("calc key: %w", err)
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
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
