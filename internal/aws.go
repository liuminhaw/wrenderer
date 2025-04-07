package internal

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go/aws"
)

type Queue struct {
	Client   *sqs.Client
	Url string
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
