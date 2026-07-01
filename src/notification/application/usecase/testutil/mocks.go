package testutil

import (
	"context"

	"notifications/src/notification/application/request"
	"notifications/src/notification/application/response"
	"notifications/src/notification/application/usecase"

	"github.com/stretchr/testify/mock"
)

// MockSendNotificationUseCase mocks the send notification use case for controller tests.
type MockSendNotificationUseCase struct {
	mock.Mock
}

func NewMockSendNotificationUseCase() *MockSendNotificationUseCase {
	return &MockSendNotificationUseCase{}
}

func (m *MockSendNotificationUseCase) Execute(ctx context.Context, req *request.SendNotificationRequest) *response.SendNotificationResult {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*response.SendNotificationResult)
}

// MockGetNotificationUseCase mocks the get notification use case for controller tests.
type MockGetNotificationUseCase struct {
	mock.Mock
}

func NewMockGetNotificationUseCase() *MockGetNotificationUseCase {
	return &MockGetNotificationUseCase{}
}

func (m *MockGetNotificationUseCase) Execute(ctx context.Context, notificationID string) (*response.GetNotificationResponse, error) {
	args := m.Called(ctx, notificationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*response.GetNotificationResponse), args.Error(1)
}

// MockListNotificationsUseCase mocks the list notifications use case for controller tests.
type MockListNotificationsUseCase struct {
	mock.Mock
}

func NewMockListNotificationsUseCase() *MockListNotificationsUseCase {
	return &MockListNotificationsUseCase{}
}

func (m *MockListNotificationsUseCase) Execute(ctx context.Context, req *request.ListNotificationsRequest) (*usecase.ListNotificationsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*usecase.ListNotificationsResponse), args.Error(1)
}
