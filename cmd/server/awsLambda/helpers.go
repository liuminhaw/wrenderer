package awsLambda

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/liuminhaw/renderer"
)

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

func sendMessageToQueue(client *sqs.Client, message string) (string, error) {
	queueUrl, exists := os.LookupEnv("SQS_WORKER_QUEUE")
	if !exists {
		return "", fmt.Errorf("missing SQS_WORKER_QUEUE environment variable")
	}

	resp, err := client.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueUrl),
		MessageBody: aws.String(message),
	})
	if err != nil {
		return "", fmt.Errorf("failed to send message to worker queue: %w", err)
	}

	return *resp.MessageId, nil
}

type respErrorMessage struct {
	Message string `json:"message"`
}

func (app *application) serverError(
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
		app.logger.Error(
			"serverError failed to marshal response message",
			slog.Any("message", message),
		)
		respBody = `{"message":"Internal server error"}`
	} else {
		app.logger.Error(
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

func (app *application) clientError(
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
		app.logger.Error(
			"serverError failed to marshal response message",
			slog.Any("message", message),
		)
		respBody = `{"message":"Internal server error"}`
	} else {
		app.logger.Error(
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
