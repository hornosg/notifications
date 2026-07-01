package testutil

import (
	"context"

	"notifications/src/notification/domain"

	"github.com/stretchr/testify/mock"
)

// MockNotificationRepository implementa port.NotificationRepository para tests.
type MockNotificationRepository struct {
	mock.Mock
}

func NewMockNotificationRepository() *MockNotificationRepository {
	return &MockNotificationRepository{}
}

func (m *MockNotificationRepository) Save(ctx context.Context, notification *domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) FindByID(ctx context.Context, namespace, tenantID, id string) (*domain.Notification, error) {
	args := m.Called(ctx, namespace, tenantID, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) Update(ctx context.Context, notification *domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) UpdateStatus(ctx context.Context, namespace, tenantID, id string, status domain.NotificationStatus, errMsg string) error {
	args := m.Called(ctx, namespace, tenantID, id, status, errMsg)
	return args.Error(0)
}

func (m *MockNotificationRepository) FindPendingNotifications(ctx context.Context, namespace, tenantID string) ([]*domain.Notification, error) {
	args := m.Called(ctx, namespace, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) FindByFilters(ctx context.Context, filters domain.NotificationFilters) ([]*domain.Notification, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) ExistsByDedupKey(ctx context.Context, namespace, tenantID, dedupKey string) (bool, error) {
	args := m.Called(ctx, namespace, tenantID, dedupKey)
	return args.Bool(0), args.Error(1)
}

// MockTemplateRepository implementa port.TemplateRepository para tests.
type MockTemplateRepository struct {
	mock.Mock
}

func NewMockTemplateRepository() *MockTemplateRepository {
	return &MockTemplateRepository{}
}

func (m *MockTemplateRepository) FindByID(ctx context.Context, id string) (*domain.Template, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) FindByName(ctx context.Context, name string) (*domain.Template, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) FindByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType) (*domain.Template, error) {
	args := m.Called(ctx, action, notificationType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Template), args.Error(1)
}

func (m *MockTemplateRepository) Save(ctx context.Context, template *domain.Template) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

func (m *MockTemplateRepository) Update(ctx context.Context, template *domain.Template) error {
	args := m.Called(ctx, template)
	return args.Error(0)
}

// MockEmailSender implementa port.EmailSender para tests.
type MockEmailSender struct {
	mock.Mock
}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{}
}

func (m *MockEmailSender) SendEmail(ctx context.Context, to string, templateID string, data map[string]interface{}) error {
	args := m.Called(ctx, to, templateID, data)
	return args.Error(0)
}

func (m *MockEmailSender) SendEmailByAction(ctx context.Context, to string, action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) error {
	args := m.Called(ctx, to, action, notificationType, data)
	return args.Error(0)
}

func (m *MockEmailSender) ValidateEmail(email string) bool {
	args := m.Called(email)
	return args.Bool(0)
}

// MockQueue implementa port.Queue para tests.
type MockQueue struct {
	mock.Mock
}

func NewMockQueue() *MockQueue {
	return &MockQueue{}
}

func (m *MockQueue) Enqueue(ctx context.Context, notification *domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockQueue) Dequeue(ctx context.Context) (*domain.Notification, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockQueue) Size(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}
