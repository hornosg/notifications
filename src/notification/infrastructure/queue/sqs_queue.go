package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"go.uber.org/zap"
)

type sqsQueue struct {
	client   *sqs.SQS
	queueURL string
	logger   *zap.Logger
}

type SQSConfig struct {
	QueueURL string
	Region   string
}

// NewSQSQueue crea una nueva instancia de cola SQS.
func NewSQSQueue(config SQSConfig) (port.Queue, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(config.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	return &sqsQueue{
		client:   sqs.New(sess),
		queueURL: config.QueueURL,
		logger:   logger.GetLogger(),
	}, nil
}

func (q *sqsQueue) Enqueue(ctx context.Context, notification *domain.Notification) error {
	q.logger.Info("Enqueueing notification to SQS",
		zap.String("notification_id", notification.ID), zap.String("queue_url", q.queueURL))

	requestFormat := map[string]interface{}{
		"notification_id": notification.ID,
		"namespace":       notification.Namespace,
		"tenant_id":       notification.TenantID,
		"type":            string(notification.Type),
		"action":          string(notification.Action),
		"recipient":       notification.Recipient,
		"data":            notification.Data,
	}

	messageBody, err := json.Marshal(requestFormat)
	if err != nil {
		q.logger.Error("Failed to marshal notification request for SQS",
			zap.String("notification_id", notification.ID), zap.Error(err))
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	input := &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(messageBody)),
		MessageAttributes: map[string]*sqs.MessageAttributeValue{
			"notification_id": {DataType: aws.String("String"), StringValue: aws.String(notification.ID)},
			"namespace":       {DataType: aws.String("String"), StringValue: aws.String(notification.Namespace)},
			"tenant_id":       {DataType: aws.String("String"), StringValue: aws.String(notification.TenantID)},
			"action":          {DataType: aws.String("String"), StringValue: aws.String(string(notification.Action))},
			"type":            {DataType: aws.String("String"), StringValue: aws.String(string(notification.Type))},
		},
	}

	result, err := q.client.SendMessageWithContext(ctx, input)
	if err != nil {
		q.logger.Error("Failed to send message to SQS",
			zap.String("notification_id", notification.ID), zap.String("queue_url", q.queueURL), zap.Error(err))
		return fmt.Errorf("failed to send message to SQS: %w", err)
	}

	q.logger.Info("Message sent to SQS successfully",
		zap.String("notification_id", notification.ID), zap.String("message_id", *result.MessageId))
	return nil
}

// queueMessage es el formato JSON neutro que viaja por SQS, desacoplado del DTO HTTP.
type queueMessage struct {
	NotificationID string                 `json:"notification_id"`
	Namespace      string                 `json:"namespace"`
	TenantID       string                 `json:"tenant_id"`
	Type           string                 `json:"type"`
	Action         string                 `json:"action"`
	Recipient      string                 `json:"recipient"`
	Data           map[string]interface{} `json:"data"`
}

func (q *sqsQueue) Dequeue(ctx context.Context) (*domain.Notification, error) {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:               aws.String(q.queueURL),
		MaxNumberOfMessages:    aws.Int64(1),
		WaitTimeSeconds:        aws.Int64(20),
		MessageAttributeNames:  []*string{aws.String("All")},
	}

	result, err := q.client.ReceiveMessageWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to receive message from SQS: %w", err)
	}
	if len(result.Messages) == 0 {
		return nil, nil
	}

	message := result.Messages[0]

	var msg queueMessage
	if err := json.Unmarshal([]byte(*message.Body), &msg); err != nil {
		q.logger.Error("Failed to unmarshal notification message from SQS",
			zap.String("message_body", *message.Body), zap.Error(err))
		if deleteErr := q.deleteMessage(ctx, message.ReceiptHandle); deleteErr != nil {
			q.logger.Error("Failed to delete malformed message", zap.Error(deleteErr))
		}
		return nil, fmt.Errorf("failed to unmarshal notification message: %w", err)
	}

	var notificationID string
	if attrs := message.MessageAttributes; attrs != nil {
		if idAttr := attrs["notification_id"]; idAttr != nil && idAttr.StringValue != nil {
			notificationID = *idAttr.StringValue
		}
	}
	if msg.NotificationID != "" {
		notificationID = msg.NotificationID
	}

	notification := &domain.Notification{
		ID:        notificationID,
		Namespace: msg.Namespace,
		TenantID:  msg.TenantID,
		Type:      domain.NotificationType(msg.Type),
		Action:    domain.NotificationAction(msg.Action),
		Recipient: msg.Recipient,
		Data:      msg.Data,
		Status:    domain.StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	q.logger.Info("Message dequeued from SQS successfully",
		zap.String("notification_id", notificationID), zap.String("message_id", *message.MessageId))

	if err := q.deleteMessage(ctx, message.ReceiptHandle); err != nil {
		q.logger.Error("Failed to delete message from SQS after processing",
			zap.String("notification_id", notificationID), zap.Error(err))
	}

	return notification, nil
}

func (q *sqsQueue) Size(ctx context.Context) (int64, error) {
	input := &sqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(q.queueURL),
		AttributeNames: []*string{aws.String("ApproximateNumberOfMessages")},
	}

	result, err := q.client.GetQueueAttributesWithContext(ctx, input)
	if err != nil {
		q.logger.Error("Failed to get queue attributes", zap.String("queue_url", q.queueURL), zap.Error(err))
		return 0, fmt.Errorf("failed to get queue attributes: %w", err)
	}

	if countStr, exists := result.Attributes["ApproximateNumberOfMessages"]; exists {
		var count int64
		if _, err := fmt.Sscanf(*countStr, "%d", &count); err != nil {
			q.logger.Error("Failed to parse message count", zap.Error(err))
			return 0, fmt.Errorf("failed to parse message count: %w", err)
		}
		return count, nil
	}

	return 0, fmt.Errorf("ApproximateNumberOfMessages attribute not found")
}

func (q *sqsQueue) deleteMessage(ctx context.Context, receiptHandle *string) error {
	input := &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: receiptHandle,
	}
	_, err := q.client.DeleteMessageWithContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete message: %w", err)
	}
	return nil
}
