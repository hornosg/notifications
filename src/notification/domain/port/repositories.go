package port

import (
	"context"

	"notifications/src/notification/domain"
)

// NotificationRepository persiste y recupera notificaciones.
type NotificationRepository interface {
	Save(ctx context.Context, notification *domain.Notification) error
	FindByID(ctx context.Context, namespace, tenantID, id string) (*domain.Notification, error)
	Update(ctx context.Context, notification *domain.Notification) error
	UpdateStatus(ctx context.Context, namespace, tenantID, id string, status domain.NotificationStatus, error string) error
	FindPendingNotifications(ctx context.Context, namespace, tenantID string) ([]*domain.Notification, error)
	FindByFilters(ctx context.Context, filters domain.NotificationFilters) ([]*domain.Notification, error)
	// ExistsByDedupKey es el backstop de idempotencia en DB.
	ExistsByDedupKey(ctx context.Context, namespace, tenantID, dedupKey string) (bool, error)
}

// TemplateRepository persiste y recupera templates.
type TemplateRepository interface {
	FindByID(ctx context.Context, id string) (*domain.Template, error)
	FindByName(ctx context.Context, name string) (*domain.Template, error)
	FindByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType) (*domain.Template, error)
	Save(ctx context.Context, template *domain.Template) error
	Update(ctx context.Context, template *domain.Template) error
}

// TemplateService renderiza templates por ID o por acción/tipo.
type TemplateService interface {
	RenderTemplateByAction(action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) (subject string, html string, err error)
	RenderTemplate(templateID string, data map[string]interface{}) (subject string, html string, err error)
	GetTemplate(templateID string) (*domain.Template, error)
	GetTemplateByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType) (*domain.Template, error)
}

// ProjectConfigRepository lee/escribe la configuración por proyecto.
type ProjectConfigRepository interface {
	FindByNamespace(ctx context.Context, namespace string) (*domain.ProjectConfig, error)
	Save(ctx context.Context, config *domain.ProjectConfig) error
	Update(ctx context.Context, config *domain.ProjectConfig) error
}
