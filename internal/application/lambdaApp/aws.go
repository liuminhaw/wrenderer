package lambdaApp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/smithy-go"
)

// CheckObjectExists checks if an object exists in S3 bucket or if the object is empty
// Returns true if the object exists and false if it does not exist
// Error is returned if there is an error checking the object
func CheckObjectExists(client *s3.Client, objectKey string) (bool, error) {
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

// func clearDomainCache(client *s3.Client, domain string) error {
// 	render, err := wrender.NewWrender(domain)
// 	if err != nil {
// 		return fmt.Errorf("clearDomainCache: %w", err)
// 	}
//
// 	prefix := fmt.Sprintf("%s/", render.GetPrefixPath())
// 	if err := deletePrefixFromS3(client, prefix); err != nil {
// 		return fmt.Errorf("clearDomainCache: delete domain cache: %w", err)
// 	}
//
// 	return nil
// }

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

// func clearUrlCache(client *s3.Client, urlParam string) error {
// 	render, err := wrender.NewWrender(urlParam)
// 	if err != nil {
// 		return fmt.Errorf("clearUrlCache: new wrender: %w", err)
// 	}
//
// 	// Remove the object from s3
// 	if err := deleteObjectFromS3(client, render.CachePath); err != nil {
// 		return fmt.Errorf("clearUrlCache: delete object: %w", err)
// 	}
// 	// Remove the host prefix if no more objects are left
// 	prefix := fmt.Sprintf("%s/", render.GetPrefixPath())
// 	empty, err := checkDomainEmpty(client, prefix)
// 	if err != nil {
// 		return fmt.Errorf("clearUrlCache: check empty domain cache: %w", err)
// 	}
// 	if empty {
// 		if err := deletePrefixFromS3(client, prefix); err != nil {
// 			return fmt.Errorf("clearUrlCache: delete domain cache: %w", err)
// 		}
// 	}
//
// 	return nil
// }

// deleteObjectFromS3 deletes an object from S3 bucket with given object key
func deleteObjectFromS3(client *s3.Client, objectKey string) error {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return fmt.Errorf("S3_BUCKET_NAME environment variable is not set")
	}

	_, err := client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(s3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
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
