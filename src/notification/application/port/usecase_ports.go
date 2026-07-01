package port

import (
	"context"

	"notifications/src/notification/application/request"
	"notifications/src/notification/application/response"
	"notifications/src/notification/application/usecase"
)

// SendNotificationUseCase expone el caso de uso de envío de notificaciones.
type SendNotificationUseCase interface {
	Execute(ctx context.Context, req *request.SendNotificationRequest) *response.SendNotificationResult
}

// GetNotificationUseCase expone el caso de uso de consulta de notificación por ID.
type GetNotificationUseCase interface {
	Execute(ctx context.Context, notificationID string) (*response.GetNotificationResponse, error)
}

// ListNotificationsUseCase expone el caso de uso de listado de notificaciones.
type ListNotificationsUseCase interface {
	Execute(ctx context.Context, req *request.ListNotificationsRequest) (*usecase.ListNotificationsResponse, error)
}
