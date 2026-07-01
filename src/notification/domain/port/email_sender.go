package port

import (
	"context"

	"notifications/src/notification/domain"
)

type EmailSender interface {
	SendEmail(ctx context.Context, to string, templateID string, data map[string]interface{}) error
	SendEmailByAction(ctx context.Context, to string, action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) error
	ValidateEmail(email string) bool
}
