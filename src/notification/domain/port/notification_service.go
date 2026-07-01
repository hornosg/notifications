package port

import (
	"context"

	"notifications/src/notification/domain"
)

// NotificationService es el puerto de entrada de alto nivel para operar sobre notificaciones.
type NotificationService interface {
	SendNotification(ctx context.Context, notification *domain.Notification) error
	SendNotificationAsync(ctx context.Context, notification *domain.Notification) error
	RetryFailedNotifications(ctx context.Context) error
	GetNotificationStatus(ctx context.Context, id string) (*domain.Notification, error)
}
