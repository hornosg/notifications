package port

// NotificationEvent es el payload canónico para eventos de dominio de notificaciones (ADR-001).
// Campos flat, named. Los nombres comunes (tenant_id, user_id) son idénticos al resto
// de la flota para que el LogQL cross-service funcione. Todos opcionales salvo Event.
//
// PRIVACIDAD: NO incluir destinatario (email/teléfono) ni contenido del mensaje en claro.
// Solo IDs, tipo de notificación, canal y resultado.
type NotificationEvent struct {
	Event            string // <domain>.<action>_<result>, p.ej. "notification.sent"
	TenantID         string
	UserID           string
	NotificationID   string
	NotificationType string // canal: "email", "sms"
	Action           string // WELCOME, PASSWORD_RESET, etc.
	Reason           string // mensaje de error, solo en casos de fallo
}

// NotificationEventLogger es el puerto para emitir eventos canónicos de notificaciones.
// El código de aplicación depende de esta interfaz; el adapter (JSON a stdout,
// Loki push, etc.) la implementa. Nunca al revés.
type NotificationEventLogger interface {
	Log(e NotificationEvent)
}
