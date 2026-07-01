package request

import (
	"errors"
	"notifications/src/notification/domain"
)

type SendNotificationRequest struct {
	NotificationID string                 `json:"notification_id,omitempty"` // Solo para SQS
	Namespace      string                 `json:"namespace,omitempty"`       // proyecto (IDP). Default 'mc'.
	TenantID       string                 `json:"tenant_id,omitempty"`       // tenant dentro del proyecto.
	Type           string                 `json:"type" binding:"required"`
	Action         string                 `json:"action" binding:"required"`
	Recipient      string                 `json:"recipient" binding:"required"`
	Data           map[string]interface{} `json:"data"`
	Async          bool                   `json:"async"`
	DedupKey       string                 `json:"dedup_key,omitempty"` // idempotencia: event_id o Idempotency-Key. Vacío = sin dedup.
}

// Validate realiza validaciones adicionales de negocio
func (r *SendNotificationRequest) Validate() error {
	// Validar que la acción sea válida
	action := domain.NotificationAction(r.Action)
	if !domain.IsValidAction(action) {
		return errors.New("acción no válida")
	}

	return nil
}
