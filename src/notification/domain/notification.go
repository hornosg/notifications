package domain

import (
	"time"
)

type NotificationType string
type NotificationStatus string
type NotificationAction string

const (
	EmailNotification NotificationType = "email"
	SMSNotification   NotificationType = "sms"

	StatusPending  NotificationStatus = "pending"
	StatusSent     NotificationStatus = "sent"
	StatusFailed   NotificationStatus = "failed"
	StatusRetrying NotificationStatus = "retrying"
	StatusQueued   NotificationStatus = "queued"

	// Acciones de notificación
	ActionWelcome              NotificationAction = "WELCOME"
	ActionEmailVerification    NotificationAction = "EMAIL_VERIFICATION"
	ActionPasswordReset        NotificationAction = "PASSWORD_RESET"
	ActionOrderConfirmation    NotificationAction = "ORDER_CONFIRMATION"
	ActionShippingNotification NotificationAction = "SHIPPING_NOTIFICATION"
	ActionOrderCancellation    NotificationAction = "ORDER_CANCELLATION"
	ActionPaymentReminder      NotificationAction = "PAYMENT_REMINDER"
)

type Notification struct {
	ID         string
	Namespace  string // proyecto (IDP). Default 'mc'. Scope de nivel superior.
	TenantID   string // tenant dentro del proyecto (puede ser vacío para notifs de plataforma).
	Type       NotificationType
	Action     NotificationAction
	TemplateID string
	Recipient  string
	Data       map[string]interface{}
	Status     NotificationStatus
	RetryCount int
	Error      string
	DedupKey   string // idempotencia: event_id (eventos) o Idempotency-Key/hash (API sync). Vacío = sin dedup.
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ValidActions devuelve las acciones válidas
func ValidActions() []NotificationAction {
	return []NotificationAction{
		ActionWelcome,
		ActionEmailVerification,
		ActionPasswordReset,
		ActionOrderConfirmation,
		ActionShippingNotification,
		ActionOrderCancellation,
		ActionPaymentReminder,
	}
}

// IsValidAction verifica si una acción es válida
func IsValidAction(action NotificationAction) bool {
	for _, validAction := range ValidActions() {
		if action == validAction {
			return true
		}
	}
	return false
}

// NotificationFilters para filtrar notificaciones.
// No lleva Namespace/TenantID: el scope lo aplica RLS solo, vía la conexión con las GUC
// de sesión ya fijadas por database.TenantSession (decisión E23 2026-07-01).
type NotificationFilters struct {
	Type      *NotificationType   `json:"type,omitempty"`
	Action    *NotificationAction `json:"action,omitempty"`
	Recipient *string             `json:"recipient,omitempty"`
	Status    *NotificationStatus `json:"status,omitempty"`
	Limit     int                 `json:"limit,omitempty"`
	Offset    int                 `json:"offset,omitempty"`
}
