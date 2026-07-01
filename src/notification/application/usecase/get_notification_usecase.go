package usecase

import (
	"context"
	"errors"

	"notifications/src/notification/application/response"
	"notifications/src/notification/domain/port"
)

var (
	ErrNotificationNotFound = errors.New("notification not found")
)

type GetNotificationUseCase struct {
	notificationRepo port.NotificationRepository
}

func NewGetNotificationUseCase(
	notificationRepo port.NotificationRepository,
) *GetNotificationUseCase {
	return &GetNotificationUseCase{
		notificationRepo: notificationRepo,
	}
}

func (uc *GetNotificationUseCase) Execute(ctx context.Context, notificationID string) (*response.GetNotificationResponse, error) {
	if uc.notificationRepo == nil {
		return nil, ErrNotificationNotFound
	}

	// namespace/tenant ya vienen resueltos en la conexión (RLS vía database.TenantSession);
	// no se filtra acá — decisión E23 2026-07-01.
	notification, err := uc.notificationRepo.FindByID(ctx, notificationID)
	if err != nil {
		return nil, ErrNotificationNotFound
	}

	return &response.GetNotificationResponse{
		ID:        notification.ID,
		Type:      string(notification.Type),
		Recipient: notification.Recipient,
		Status:    string(notification.Status),
		Namespace: notification.Namespace,
		TenantID:  notification.TenantID,
		Data:      notification.Data,
		CreatedAt: notification.CreatedAt,
		UpdatedAt: notification.UpdatedAt,
	}, nil
}
