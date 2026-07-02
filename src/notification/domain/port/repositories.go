package port

import (
	"context"

	"notifications/src/notification/domain"
)

// NotificationRepository persiste y recupera notificaciones.
//
// Los métodos de lectura NO reciben namespace/tenantID: la conexión que usa la
// implementación abre su propia transacción vía go-shared postgres.WithRLSInTransaction
// (SET LOCAL, PROP-007), que fija esos valores como GUC de sesión, y las policies RLS
// (002_rls.sql) filtran solas.
// No repetir el filtro en la query es una decisión explícita — confiar en RLS como única
// fuente de aislamiento (decisión E23 2026-07-01), no defensa en profundidad con doble filtro.
type NotificationRepository interface {
	Save(ctx context.Context, notification *domain.Notification) error
	FindByID(ctx context.Context, id string) (*domain.Notification, error)
	Update(ctx context.Context, notification *domain.Notification) error
	UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus, error string) error
	FindPendingNotifications(ctx context.Context) ([]*domain.Notification, error)
	FindByFilters(ctx context.Context, filters domain.NotificationFilters) ([]*domain.Notification, error)
	// ExistsByDedupKey es el backstop de idempotencia en DB.
	ExistsByDedupKey(ctx context.Context, dedupKey string) (bool, error)
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
//
// Todos los métodos que pueden tocar TemplateRepository reciben ctx: el legacy
// notification-service resolvía el namespace abriendo su propia transacción con
// postgres.ContextWithRLS(context.Background(), ...), pero acá no hay tal atajo — la
// única conexión válida es la que trae los GUC de sesión ya fijados por
// go-shared postgres.WithRLSInTransaction (ver ADR-001). Sin ctx del caller no hay namespace real que
// resolver.
type TemplateService interface {
	RenderTemplateByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType, data map[string]interface{}) (subject string, html string, err error)
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
