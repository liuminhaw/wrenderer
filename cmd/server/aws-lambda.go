package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/liuminhaw/renderer"
)

type responseData struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	ObjectKey string `json:"objectKey"`
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

	// Render the page
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
		ImageLoad:      true,
		SkipFrameCount: 0,
	}
	ctx := context.Background()
	ctx = renderer.WithBrowserContext(ctx, &browserContext)
	ctx = renderer.WithRendererContext(ctx, &rendererContext)

	content, err := renderer.RenderPage(ctx, urlParam)
	if err != nil {
		log.Printf("Render Failed: %s", err)
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Render Failed",
		}, nil
	}

	// Regular expressions for matching base64 images and SVG content
	// regexpBase64 := regexp.MustCompile(`"data:image\/.*?;base64.*?"`)
	// regexpSVG := regexp.MustCompile(`\<svg.*?\>.*?\<\/svg\>`)

	// Replacing base64 images with empty strings and SVG content with empty <svg></svg> tags
	// newContext := regexpBase64.ReplaceAllString(string(content), `""`)
	// newContext = regexpSVG.ReplaceAllString(newContext, `<svg></svg>`)

	// TODO: Upload rendered result to S3
	contentReader := bytes.NewReader(content)
	var objectPathKey string
	if responseBodyData.Port != "" {
		objectPathKey = strings.Join([]string{responseBodyData.Host, responseBodyData.Port}, "_")
		objectPathKey = strings.Join([]string{objectPathKey, responseBodyData.ObjectKey}, "/")
	} else {
		objectPathKey = strings.Join([]string{responseBodyData.Host, responseBodyData.ObjectKey}, "/")
	}
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
