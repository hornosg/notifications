package testutil

import (
	"time"

	"notifications/src/notification/domain"
)

// NotificationMother crea instancias de Notification para tests
type NotificationMother struct{}

func NewNotificationMother() *NotificationMother {
	return &NotificationMother{}
}

func (m *NotificationMother) Default() *domain.Notification {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	return &domain.Notification{
		ID:         "notif-001",
		Type:       domain.EmailNotification,
		Action:     domain.ActionWelcome,
		TemplateID: "welcome_email",
		Recipient:  "user@example.com",
		Data:       map[string]interface{}{"name": "Juan"},
		Status:     domain.StatusPending,
		RetryCount: 0,
		Error:      "",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func (m *NotificationMother) WithStatus(status domain.NotificationStatus) *domain.Notification {
	n := m.Default()
	n.Status = status
	return n
}

func (m *NotificationMother) WithAction(action domain.NotificationAction) *domain.Notification {
	n := m.Default()
	n.Action = action
	return n
}

func (m *NotificationMother) WithRecipient(recipient string) *domain.Notification {
	n := m.Default()
	n.Recipient = recipient
	return n
}

func (m *NotificationMother) Sent() *domain.Notification {
	return m.WithStatus(domain.StatusSent)
}

func (m *NotificationMother) Failed() *domain.Notification {
	n := m.WithStatus(domain.StatusFailed)
	n.Error = "SMTP connection refused"
	n.RetryCount = 1
	return n
}

func (m *NotificationMother) Queued() *domain.Notification {
	return m.WithStatus(domain.StatusQueued)
}

// TemplateMother crea instancias de Template para tests
type TemplateMother struct{}

func NewTemplateMother() *TemplateMother {
	return &TemplateMother{}
}

func (m *TemplateMother) Default() *domain.Template {
	now := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	return &domain.Template{
		ID:        "tpl-001",
		Name:      "welcome_email",
		Subject:   "Bienvenido a nuestra plataforma!",
		FilePath:  "templates/email/welcome_email.html",
		Action:    domain.ActionWelcome,
		Type:      domain.EmailNotification,
		Variables: []string{"name", "email"},
		Version:   1,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (m *TemplateMother) WithAction(action domain.NotificationAction) *domain.Template {
	t := m.Default()
	t.Action = action
	return t
}

func (m *TemplateMother) Inactive() *domain.Template {
	t := m.Default()
	t.IsActive = false
	return t
}

func (m *TemplateMother) PasswordReset() *domain.Template {
	t := m.Default()
	t.ID = "tpl-002"
	t.Name = "password_reset"
	t.Subject = "Restablece tu contrasena"
	t.FilePath = "templates/email/password_reset.html"
	t.Action = domain.ActionPasswordReset
	t.Variables = []string{"name", "reset_link"}
	return t
}
