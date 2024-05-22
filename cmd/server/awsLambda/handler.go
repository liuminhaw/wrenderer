package awsLambda

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func LambdaHandler(event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Request path: %s", event.Path)
	log.Printf("HTTP method: %s", event.HTTPMethod)

	switch event.Path {
	case "/render":
		switch event.HTTPMethod {
		case "GET":
			return renderUrl(event)
		case "DELETE":
			fmt.Println("Delete rendered cache for")
			return deleteRenderCache(event)
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

// checkDomainEmpty checks if a bucket is empty under given prefix
// Returns true if the bucket is empty and false if it is not empty
// Error is returned if there is an error checking the bucket
func checkDomainEmpty(client *s3.Client, prefix string) (bool, error) {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return false, fmt.Errorf(
			"checkDomainEmpty: S3_BUCKET_NAME environment variable is not set",
		)
	}

	result, err := client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket:  aws.String(s3BucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("checkDomainEmpty: %w", err)
	}

	if len(result.Contents) == 0 {
		return true, nil
	}

	return false, nil
}
