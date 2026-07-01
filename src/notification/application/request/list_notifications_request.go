package request

import (
	"notifications/src/notification/domain"
)

type ListNotificationsRequest struct {
	Type      string `form:"type" json:"type,omitempty"`
	Action    string `form:"action" json:"action,omitempty"`
	Recipient string `form:"recipient" json:"recipient,omitempty"`
	Status    string `form:"status" json:"status,omitempty"`
	Page      int    `form:"page" json:"page,omitempty"`
	Limit     int    `form:"limit" json:"limit,omitempty"`
}

// ToFilters convierte el request a domain.NotificationFilters
func (r *ListNotificationsRequest) ToFilters() domain.NotificationFilters {
	filters := domain.NotificationFilters{}

	// Convertir type si está presente
	if r.Type != "" {
		notificationType := domain.NotificationType(r.Type)
		filters.Type = &notificationType
	}

	// Convertir action si está presente
	if r.Action != "" {
		notificationAction := domain.NotificationAction(r.Action)
		filters.Action = &notificationAction
	}

	// Convertir recipient si está presente
	if r.Recipient != "" {
		filters.Recipient = &r.Recipient
	}

	// Convertir status si está presente
	if r.Status != "" {
		notificationStatus := domain.NotificationStatus(r.Status)
		filters.Status = &notificationStatus
	}

	// Configurar paginación
	page := r.Page
	if page < 1 {
		page = 1
	}

	limit := r.Limit
	if limit < 1 || limit > 100 {
		limit = 50 // Límite por defecto
	}

	filters.Limit = limit
	filters.Offset = (page - 1) * limit

	return filters
}

// Validate valida los filtros
func (r *ListNotificationsRequest) Validate() error {
	// Validar type si está presente
	if r.Type != "" {
		validType := false
		for _, t := range []string{"email", "sms"} {
			if r.Type == t {
				validType = true
				break
			}
		}
		if !validType {
			return NewValidationError("type", "debe ser 'email' o 'sms'")
		}
	}

	// Validar action si está presente
	if r.Action != "" {
		action := domain.NotificationAction(r.Action)
		if !domain.IsValidAction(action) {
			return NewValidationError("action", "acción no válida")
		}
	}

	// Validar status si está presente
	if r.Status != "" {
		validStatus := false
		for _, s := range []string{"pending", "sent", "failed", "retrying", "queued"} {
			if r.Status == s {
				validStatus = true
				break
			}
		}
		if !validStatus {
			return NewValidationError("status", "estado no válido")
		}
	}

	// Validar page
	if r.Page < 0 {
		return NewValidationError("page", "debe ser mayor a 0")
	}

	// Validar limit
	if r.Limit < 0 || r.Limit > 100 {
		return NewValidationError("limit", "debe estar entre 1 y 100")
	}

	return nil
}

// ValidationError representa un error de validación
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func NewValidationError(field, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}
