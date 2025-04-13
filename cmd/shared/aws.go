package shared

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
)

const (
	S3Service  = "s3"
	SqsService = "sqs"
)

type AwsClients struct {
	S3  *s3.Client
	Sqs *sqs.Client
}

func newAwsClients(services ...string) (*AwsClients, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, err
	}

	clients := &AwsClients{}
	for _, service := range services {
		switch service {
		case S3Service:
			clients.S3 = s3.NewFromConfig(cfg)
		case SqsService:
			clients.Sqs = sqs.NewFromConfig(cfg)
		default:
			return nil, fmt.Errorf("unsupported service: %s", service)
		}
	}

	return clients, nil
}

type ConfLoader struct {
	EnvConf EnvConfig
	Clients *AwsClients
}

func NewConfLoader(services ...string) (*ConfLoader, error) {
	envConfig, err := LambdaReadEnv()
	if err != nil {
		return nil, err
	}

	clients, err := newAwsClients(services...)
	if err != nil {
		return nil, err
	}

	return &ConfLoader{
		EnvConf: envConfig,
		Clients: clients,
	}, nil
}

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

type Queue struct {
	Client *sqs.Client
	Url    string
}

// SendMessage sends a message to the SQS queue and returns the message ID
func (q Queue) SendMessage(message string) (string, error) {
	resp, err := q.Client.SendMessage(context.Background(), &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.Url),
		MessageBody: aws.String(message),
	})
	if err != nil {
		return "", err
	}

	return *resp.MessageId, nil
}
