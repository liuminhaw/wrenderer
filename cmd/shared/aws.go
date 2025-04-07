package shared

import (
	"fmt"
	"os"
	"strconv"
)

type EnvConfig struct {
	S3BucketName         string
	S3BucketRegion       string
	JobExpirationInHours int
	SqsUrl               string
}

func LambdaReadEnv() (EnvConfig, error) {
	// S3 bucket config
	s3BucketName, ok := os.LookupEnv("S3_BUCKET_NAME")
	if !ok {
		return EnvConfig{}, fmt.Errorf("missing S3_BUCKET_NAME environment variable")
	}
	s3BucketRegion, ok := os.LookupEnv("S3_BUCKET_REGION")
	if !ok {
		return EnvConfig{}, fmt.Errorf("missing S3_BUCKET_REGION environment variable")
	}

	// Job config
	var expirationInHours int
	jobExpirationInHours, ok := os.LookupEnv("JOB_EXPIRATION_IN_HOURS")
	if !ok {
		expirationInHours = 1
	} else {
		var err error
		expirationInHours, err = strconv.Atoi(jobExpirationInHours)
		if err != nil {
			return EnvConfig{}, fmt.Errorf("JOB_EXPIRATION_IN_HOURS should be int: %w", err)
		}
	}

	// SQS queue config
	queueUrl, ok := os.LookupEnv("SQS_WORKER_QUEUE")
	if !ok {
		return EnvConfig{}, fmt.Errorf("missing SQS_WORKER_QUEUE environment variable")
	}

	return EnvConfig{
		S3BucketName:         s3BucketName,
		S3BucketRegion:       s3BucketRegion,
		JobExpirationInHours: expirationInHours,
		SqsUrl:               queueUrl,
	}, nil
}
