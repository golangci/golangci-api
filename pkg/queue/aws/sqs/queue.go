package sqs

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/golangci/golangci-api/pkg/queue"
	"github.com/golangci/golangci-shared/pkg/logutil"
	"github.com/pkg/errors"
)

type Queue struct {
	url                  string
	sqsClient            *sqs.SQS
	log                  logutil.Log
	visibilityTimeoutSec int
}

func NewQueue(url string, sess client.ConfigProvider, log logutil.Log, visibilityTimeoutSec int) *Queue {
	if sess == nil {
		return nil // TODO
	}

	return &Queue{
		url:                  url,
		sqsClient:            sqs.New(sess),
		log:                  log,
		visibilityTimeoutSec: visibilityTimeoutSec,
	}
}

func (q Queue) Put(message queue.Message) error {
	startedAt := time.Now()
	body, err := json.Marshal(message)
	if err != nil {
		return errors.Wrap(err, "can't json marshal message")
	}

	res, err := q.sqsClient.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(string(body)),
		QueueUrl:    aws.String(q.url),
	})
	if err != nil {
		return errors.Wrapf(err, "can't send message to queue (%s)", res)
	}

	q.log.Infof("Sent message with id=%s to queue for %s: %#v", *res.MessageId, time.Since(startedAt), message)
	return nil
}

func (q Queue) TryReceive() (*sqs.Message, error) {
	result, err := q.sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
		AttributeNames: []*string{
			aws.String(sqs.MessageSystemAttributeNameApproximateReceiveCount),
		},
		MessageAttributeNames: []*string{
			aws.String(sqs.QueueAttributeNameAll),
		},
		QueueUrl:            &q.url,
		MaxNumberOfMessages: aws.Int64(1),
		VisibilityTimeout:   aws.Int64(int64(q.visibilityTimeoutSec)),
		WaitTimeSeconds:     aws.Int64(20), // must be in sync with cloudformation.yml
	})
	if err != nil {
		return nil, errors.Wrap(err, "can't receive message from sqs")
	}

	if len(result.Messages) == 0 {
		return nil, nil
	}
	if len(result.Messages) != 1 {
		return nil, fmt.Errorf("invalid number of messages received from sqs: %d", len(result.Messages))
	}

	return result.Messages[0], nil
}

func (q Queue) Ack(receiptHandle string, receiveCount int, ok bool) error {
	if !ok {
		delaySec := int64((1 << uint(receiveCount)) * 60)
		if delaySec > 43200 {
			delaySec = 43200 // max allowed by aws sqs (12 hours)
		}
		_, err := q.sqsClient.ChangeMessageVisibility(&sqs.ChangeMessageVisibilityInput{
			ReceiptHandle:     aws.String(receiptHandle),
			QueueUrl:          &q.url,
			VisibilityTimeout: &delaySec,
		})
		if err != nil {
			return errors.Wrapf(err, "can't change message %s visibility for %d-th attempt to %ds",
				receiptHandle, receiveCount, delaySec)
		}

		q.log.Infof("Changed message %s visibility for %d-th attempt to %ds",
			receiptHandle, receiveCount, delaySec)
		return nil
	}

	_, err := q.sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &q.url,
		ReceiptHandle: aws.String(receiptHandle),
	})

	if err != nil {
		return errors.Wrapf(err, "can't delete message %s from queue", receiptHandle)
	}

	q.log.Infof("Deleted message %s from queue after succeeded consuming on %d-th attempt",
		receiptHandle, receiveCount)
	return nil
}
