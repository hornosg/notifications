package logging

import (
	"io"

	"notifications/src/notification/domain/port"

	sharedlog "github.com/hornosg/go-shared/infrastructure/logging"
)

// NotificationLogger implementa port.NotificationEventLogger emitiendo una línea JSON
// canónica (ADR-001) por evento, delegando el envelope en go-shared CanonicalLogger.
//
// PRIVACIDAD: el Recipient (email/teléfono) y el contenido del mensaje NUNCA se loguean.
// Solo IDs, tipo de canal, acción y resultado.
type NotificationLogger struct {
	canonical *sharedlog.CanonicalLogger
}

// NewNotificationLogger crea el adapter escribiendo a stdout. El service se fija acá, nunca por-call.
func NewNotificationLogger(service string) *NotificationLogger {
	return &NotificationLogger{canonical: sharedlog.NewCanonicalLogger(service)}
}

// NewNotificationLoggerWithWriter permite inyectar un io.Writer (tests).
func NewNotificationLoggerWithWriter(service string, w io.Writer) *NotificationLogger {
	return &NotificationLogger{canonical: sharedlog.NewCanonicalLoggerWithWriter(service, w)}
}

// levelFor aplica las reglas de nivel del ADR-001 por tipo de evento.
func levelFor(event string) string {
	switch event {
	case "notification.sent", "notification.queued":
		return "info"
	case "notification.failed", "notification.send_failed", "notification.enqueue_failed",
		"notification.validation_failed", "notification.template_not_found":
		return "warn"
	case "notification.save_failed", "notification.update_failed":
		return "error"
	default:
		return "info"
	}
}

// Log traduce el NotificationEvent tipado al envelope canónico y lo emite.
func (l *NotificationLogger) Log(e port.NotificationEvent) {
	fields := map[string]any{
		"tenant_id":         e.TenantID,
		"user_id":           e.UserID,
		"notification_id":   e.NotificationID,
		"notification_type": e.NotificationType,
		"action":            e.Action,
		"reason":            e.Reason,
	}
	l.canonical.Emit(levelFor(e.Event), e.Event, fields)
}
