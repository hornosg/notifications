package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mercadocercano/eventbus"

	appctx "notifications/src/notification/application/appcontext"
	"notifications/src/notification/application/request"
	"notifications/src/notification/application/response"
	"notifications/src/notification/domain/port"
)

// NotificationSender es el subconjunto del use case de envío que el handler necesita.
type NotificationSender interface {
	Execute(ctx context.Context, req *request.SendNotificationRequest) *response.SendNotificationResult
}

// TenantRegisteredPayload es el payload de `onboarding.tenant.registered` v1.
type TenantRegisteredPayload struct {
	Namespace string                 `json:"namespace"`
	TenantID  string                 `json:"tenant_id"`
	UserID    string                 `json:"user_id"`
	Type      string                 `json:"type"`
	Action    string                 `json:"action"`
	Recipient string                 `json:"recipient"`
	Data      map[string]interface{} `json:"data"`
}

// NotificationEventHandler consume eventos de dominio del EventBus y los mapea a
// notificaciones (patrón copiado de ledger-service: ConsumerName() + Handle()).
//
// Corre fuera del ciclo HTTP, pero eso ya no requiere nada especial (PROP-007): cada
// repository que toca sender.Execute abre su propia transacción vía go-shared
// postgres.WithRLSInTransaction. Este handler solo tiene que dejar namespace/tenant en el
// contexto (appctx.WithRLS) antes de invocar al use case.
type NotificationEventHandler struct {
	sender      NotificationSender
	eventLogger port.NotificationEventLogger
}

// NewNotificationEventHandler crea el handler. eventLogger puede ser nil (nil-safe).
func NewNotificationEventHandler(sender NotificationSender, eventLogger port.NotificationEventLogger) *NotificationEventHandler {
	return &NotificationEventHandler{sender: sender, eventLogger: eventLogger}
}

// ConsumerName identifica al consumidor en el event_consumers del EventBus.
func (h *NotificationEventHandler) ConsumerName() string {
	return "notifications"
}

func (h *NotificationEventHandler) logEvent(e port.NotificationEvent) {
	if h.eventLogger != nil {
		h.eventLogger.Log(e)
	}
}

// Handle rutea por tipo de evento. Eventos desconocidos se ack-ean (return nil) para no
// bloquear el cursor del worker con eventos de otros dominios.
func (h *NotificationEventHandler) Handle(ctx context.Context, event eventbus.DomainEvent) error {
	switch event.EventType() {
	case "onboarding.tenant.registered":
		return h.handleTenantRegistered(ctx, event)
	default:
		h.logEvent(port.NotificationEvent{
			Event:  "notification.event_unknown",
			Reason: "unknown event type: " + event.EventType(),
		})
		return nil
	}
}

// handleTenantRegistered → email de bienvenida (WELCOME). dedup_key = event.ID() para que
// la entrega at-least-once del EventBus no genere correos duplicados.
func (h *NotificationEventHandler) handleTenantRegistered(ctx context.Context, event eventbus.DomainEvent) error {
	var p TenantRegisteredPayload
	if err := json.Unmarshal(event.Payload(), &p); err != nil {
		h.logEvent(port.NotificationEvent{Event: "notification.event_parse_failed", Reason: err.Error()})
		return nil // payload corrupto: ack, reintentar no lo arregla (poison message)
	}

	if p.Recipient == "" {
		h.logEvent(port.NotificationEvent{
			Event: "notification.event_skipped", TenantID: p.TenantID, UserID: p.UserID, Reason: "missing recipient",
		})
		return nil
	}

	notifType := p.Type
	if notifType == "" {
		notifType = "email"
	}
	action := p.Action
	if action == "" {
		action = "WELCOME"
	}
	namespace := p.Namespace
	if namespace == "" {
		namespace = "mc"
	}

	req := &request.SendNotificationRequest{
		Namespace: namespace,
		TenantID:  p.TenantID,
		Type:      notifType,
		Action:    action,
		Recipient: p.Recipient,
		Data:      p.Data,
		Async:     false,
		DedupKey:  event.ID(),
	}

	h.logEvent(port.NotificationEvent{
		Event: "notification.event_consumed", TenantID: p.TenantID, UserID: p.UserID,
		NotificationType: notifType, Action: action,
	})

	scopedCtx := appctx.WithRLS(ctx, namespace, p.TenantID)
	result := h.sender.Execute(scopedCtx, req)

	if result != nil && !result.Success {
		reason := "unknown"
		if result.Error != nil {
			reason = result.Error.Code
		}
		h.logEvent(port.NotificationEvent{
			Event: "notification.event_consume_failed", TenantID: p.TenantID, UserID: p.UserID,
			NotificationType: notifType, Action: action, Reason: reason,
		})
		// 5xx = transitorio → error para que el EventBus reintente. 4xx = permanente → ack.
		if result.HTTPStatus >= 500 {
			return fmt.Errorf("notification send failed (transient): %s", reason)
		}
		return nil
	}

	return nil
}
