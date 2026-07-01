package usecase

import (
	"context"
	"fmt"

	"notifications/src/notification/application/request"
	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	"go.uber.org/zap"
)

type ListNotificationsUseCase struct {
	notificationRepo port.NotificationRepository
	logger           *zap.Logger
}

// NewListNotificationsUseCase crea una nueva instancia del caso de uso
func NewListNotificationsUseCase(notificationRepo port.NotificationRepository) *ListNotificationsUseCase {
	return &ListNotificationsUseCase{
		notificationRepo: notificationRepo,
		logger:           logger.GetLogger(),
	}
}

// Execute ejecuta el caso de uso para listar notificaciones
func (uc *ListNotificationsUseCase) Execute(ctx context.Context, req *request.ListNotificationsRequest) (*ListNotificationsResponse, error) {
	uc.logger.Info("Listing notifications",
		zap.String("type", req.Type),
		zap.String("action", req.Action),
		zap.String("recipient", req.Recipient),
		zap.String("status", req.Status),
		zap.Int("page", req.Page),
		zap.Int("limit", req.Limit))

	// Validar request
	if err := req.Validate(); err != nil {
		uc.logger.Warn("Invalid list notifications request", zap.Error(err))
		return nil, fmt.Errorf("solicitud inválida: %w", err)
	}

	// Verificar que el repositorio esté disponible
	if uc.notificationRepo == nil {
		uc.logger.Error("Notification repository is nil")
		return nil, fmt.Errorf("servicio de base de datos no disponible")
	}

	// Convertir request a filtros. namespace/tenant NO se inyectan acá: los aplica RLS
	// solo, vía la conexión con las GUC de sesión ya fijadas — decisión E23 2026-07-01.
	filters := req.ToFilters()

	// Buscar notificaciones
	notifications, err := uc.notificationRepo.FindByFilters(ctx, filters)
	if err != nil {
		uc.logger.Error("Error finding notifications by filters",
			zap.Error(err),
			zap.Any("filters", filters))
		return nil, fmt.Errorf("error buscando notificaciones: %w", err)
	}

	// Convertir a DTOs de respuesta
	notificationDTOs := make([]NotificationDTO, len(notifications))
	for i, notification := range notifications {
		notificationDTOs[i] = ToNotificationDTO(notification)
	}

	// Calcular metadatos de paginación
	pagination := PaginationMetadata{
		Page:    req.Page,
		Limit:   req.Limit,
		Total:   len(notificationDTOs),
		HasMore: len(notificationDTOs) == req.Limit, // Si retorna el límite completo, probablemente hay más
	}

	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.Limit < 1 {
		pagination.Limit = 50
	}

	response := &ListNotificationsResponse{
		Notifications: notificationDTOs,
		Pagination:    pagination,
		Filters: FiltersSummary{
			Type:      req.Type,
			Action:    req.Action,
			Recipient: req.Recipient,
			Status:    req.Status,
		},
	}

	uc.logger.Info("Notifications listed successfully",
		zap.Int("count", len(notificationDTOs)),
		zap.Int("page", pagination.Page),
		zap.Int("limit", pagination.Limit))

	return response, nil
}

// ListNotificationsResponse representa la respuesta del listado
type ListNotificationsResponse struct {
	Notifications []NotificationDTO  `json:"notifications"`
	Pagination    PaginationMetadata `json:"pagination"`
	Filters       FiltersSummary     `json:"filters"`
}

// NotificationDTO representa una notificación en la respuesta
type NotificationDTO struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Action    string                 `json:"action"`
	Recipient string                 `json:"recipient"`
	Status    string                 `json:"status"`
	Data      map[string]interface{} `json:"data"`
	CreatedAt string                 `json:"created_at"`
	UpdatedAt string                 `json:"updated_at"`
}

// PaginationMetadata contiene información de paginación
type PaginationMetadata struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasMore bool `json:"has_more"`
}

// FiltersSummary muestra los filtros aplicados
type FiltersSummary struct {
	Type      string `json:"type,omitempty"`
	Action    string `json:"action,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Status    string `json:"status,omitempty"`
}

// ToNotificationDTO convierte una notificación del dominio a DTO
func ToNotificationDTO(notification *domain.Notification) NotificationDTO {
	return NotificationDTO{
		ID:        notification.ID,
		Type:      string(notification.Type),
		Action:    string(notification.Action),
		Recipient: notification.Recipient,
		Status:    string(notification.Status),
		Data:      notification.Data,
		CreatedAt: notification.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpdatedAt: notification.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
