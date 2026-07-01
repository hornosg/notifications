package usecase

import (
	"context"
	"errors"

	appctx "notifications/src/notification/application/appcontext"
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

	namespace := appctx.NamespaceFromContext(ctx)
	tenantID := appctx.TenantIDFromContext(ctx)

	notification, err := uc.notificationRepo.FindByID(ctx, namespace, tenantID, notificationID)
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
