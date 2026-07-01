package email

import (
	"context"

	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
)

// MockSender es un EmailSender de prueba que nunca envía emails reales.
type MockSender struct{}

func NewMockSender() port.EmailSender {
	return &MockSender{}
}

func (m *MockSender) SendEmail(ctx context.Context, to string, templateID string, data map[string]interface{}) error {
	return nil
}

func (m *MockSender) SendEmailByAction(ctx context.Context, to string, action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) error {
	return nil
}

func (m *MockSender) ValidateEmail(email string) bool {
	return true
}
