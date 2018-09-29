package consumer

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golangci/golangci-shared/pkg/config"
	"github.com/pkg/errors"

	"github.com/aws/aws-lambda-go/events"
	awslambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	awssqs "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/golangci/golangci-api/pkg/queue/aws/sqs"
	"github.com/golangci/golangci-api/pkg/queue/consumers"
	"github.com/golangci/golangci-shared/pkg/logutil"
)

type SQS struct {
	sqsQueue             *sqs.Queue
	log                  logutil.Log
	useLambdaTrigger     bool
	consumer             consumers.Consumer
	visibilityTimeoutSec int
}

func NewSQS(log logutil.Log, cfg config.Config, sqsQueue *sqs.Queue, consumer consumers.Consumer, sqsName string, visibilityTimeoutSec int) *SQS {
	queueCfgPrefix := fmt.Sprintf("SQS_%s_QUEUE", strings.ToUpper(sqsName))
	useLambdaTrigger := cfg.GetBool(queueCfgPrefix+"_USE_LAMBDA", false)

	return &SQS{
		sqsQueue:             sqsQueue,
		log:                  log,
		useLambdaTrigger:     useLambdaTrigger,
		consumer:             consumer,
		visibilityTimeoutSec: visibilityTimeoutSec,
	}
}

func (c SQS) Run() {
	if c.useLambdaTrigger {
		c.log.Infof("Use lambda consumer")
		awslambda.Start(c.handleLambdaCall) // call blocks on net.Accept inside
		return
	}

	c.log.Infof("Use polling consumer")
	c.runPolling()
}

func (c SQS) poll() {
	message, err := c.sqsQueue.TryReceive()
	if err != nil {
		c.log.Errorf("Polling failed: %s", err)
		time.Sleep(10 * time.Second)
		return
	}

	if message == nil {
		time.Sleep(time.Second)
		return
	}

	maxTimeout := time.Second * time.Duration(c.visibilityTimeoutSec)
	timeout := time.Duration(float64(maxTimeout) * 0.8)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startedAt := time.Now()
	if err = c.onReceiveMessage(ctx, message); err != nil {
		c.log.Errorf("Failed to process received message %#v: %s", message, err)
		time.Sleep(time.Second)
		return
	}

	c.log.Infof("Polling: processed message %#v for %s", message, time.Since(startedAt))
}

func (c SQS) runPolling() {
	for {
		c.poll()
	}
}

func (c SQS) onReceiveMessage(ctx context.Context, message *awssqs.Message) error {
	if message.Body == nil {
		return errors.New("nil message body")
	}

	err := c.consumer.ConsumeMessage(ctx, []byte(*message.Body))
	if err != nil {
		c.log.Warnf("Consumer failed: %s", err)
	}
	handledOk := err == nil || errors.Cause(err) == consumers.ErrPermanent

	receiveCount := 0
	receiveCountStrPtr := message.Attributes[awssqs.MessageSystemAttributeNameApproximateReceiveCount]
	if receiveCountStrPtr == nil {
		c.log.Warnf("No receive count message attribute: %#v", message.Attributes)
	} else {
		receiveCount, err = strconv.Atoi(*receiveCountStrPtr)
		if err != nil {
			c.log.Warnf("Invalid receive count attribute %q: %s", *receiveCountStrPtr, err)
		}
	}
	if err = c.sqsQueue.Ack(*message.ReceiptHandle, receiveCount, handledOk); err != nil {
		return errors.Wrapf(err, "failed to ack message %s with receive count %d",
			*message.ReceiptHandle, receiveCount)
	}

	return nil
}

func (c SQS) handleLambdaCall(ctx context.Context, sqsEvent events.SQSEvent) error {
	if len(sqsEvent.Records) != 1 {
		return fmt.Errorf("invalid events records count %d != 1: %#v", len(sqsEvent.Records), sqsEvent.Records)
	}

	event := sqsEvent.Records[0]
	receiveCountAttr, ok := event.Attributes[awssqs.MessageSystemAttributeNameApproximateReceiveCount]
	if !ok {
		c.log.Warnf("No receive count attr: %#v", event.Attributes)
	}

	message := awssqs.Message{
		Attributes: map[string]*string{
			awssqs.MessageSystemAttributeNameApproximateReceiveCount: aws.String(receiveCountAttr),
		},
		Body: aws.String(event.Body),
	}

	if err := c.onReceiveMessage(ctx, &message); err != nil {
		c.log.Errorf("Failed to process received message %#v: %s", message, err)
		time.Sleep(time.Second)
		return nil // don't return errors, handle them by queue
	}

	c.log.Infof("Lambda trigger: processed message %#v", message)
	return nil
}
