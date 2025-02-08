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

// UploadToS3 uploads the content to S3 bucket with given object key
func UploadToS3(client *s3.Client, objectKey string, contentType string, content io.Reader) error {
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
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("uploadToS3: failed to upload object: %w", err)
	}

	return nil
}

// DeleteObjectFromS3 deletes an object from S3 bucket with given object key
func DeleteObjectFromS3(client *s3.Client, objectKey string) error {
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

func ListObjectsFromS3(client *s3.Client, prefix string) ([]string, error) {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return nil, fmt.Errorf("S3_BUCKET_NAME environment variable is not set")
	}

	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket:     aws.String(s3BucketName),
		Prefix:     aws.String(prefix),
		StartAfter: aws.String(prefix),
	})

	var objects []string
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}

		for _, obj := range page.Contents {
			objects = append(objects, *obj.Key)
		}
	}

	return objects, nil
}

func readObjectFromS3(client *s3.Client, objectKey string) ([]byte, error) {
	s3BucketName, exists := os.LookupEnv("S3_BUCKET_NAME")
	if !exists {
		return nil, fmt.Errorf("S3_BUCKET_NAME environment variable is not set")
	}

	obj, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s3BucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, err
	}
	defer obj.Body.Close()

	content, err := io.ReadAll(obj.Body)
	if err != nil {
		return nil, err
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

// checkBucketPrefixEmpty checks if a bucket is empty under given prefix
// Returns true if the bucket under given prefix is empty and
// false if it is not empty.
// Error is returned if there is an error checking the bucket
func checkBucketPrefixEmpty(client *s3.Client, prefix string) (bool, error) {
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
