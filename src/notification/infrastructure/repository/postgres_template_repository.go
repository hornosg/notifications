package repository

import (
	"database/sql"
	"fmt"
	"time"

	"context"

	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// postgresTemplateRepository sigue el mismo patrón que postgresNotificationRepository:
// opera siempre sobre la conexión RLS-scoped del contexto (database.ConnFromContext).
// templates se filtra solo por namespace (003_templates.sql) — no hay tenant_id, ver ADR-001.
type postgresTemplateRepository struct {
	logger *zap.Logger
}

func NewPostgresTemplateRepository() port.TemplateRepository {
	return &postgresTemplateRepository{
		logger: logger.GetLogger(),
	}
}

func (r *postgresTemplateRepository) FindByID(ctx context.Context, id string) (*domain.Template, error) {
	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, namespace, name, subject, file_path, action, type, version, is_active, created_at, updated_at
		FROM templates
		WHERE id = $1 AND is_active = true
	`

	var t domain.Template
	err = conn.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.Namespace, &t.Name, &t.Subject, &t.FilePath,
		&t.Action, &t.Type, &t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("Template not found", zap.String("id", id))
			return nil, fmt.Errorf("template not found: %s", id)
		}
		r.logger.Error("Error finding template by ID", zap.String("id", id), zap.Error(err))
		return nil, fmt.Errorf("error finding template: %w", err)
	}

	return &t, nil
}

func (r *postgresTemplateRepository) FindByName(ctx context.Context, name string) (*domain.Template, error) {
	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, namespace, name, subject, file_path, action, type, version, is_active, created_at, updated_at
		FROM templates
		WHERE name = $1 AND is_active = true
	`

	var t domain.Template
	err = conn.QueryRowContext(ctx, query, name).Scan(
		&t.ID, &t.Namespace, &t.Name, &t.Subject, &t.FilePath,
		&t.Action, &t.Type, &t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("Template not found", zap.String("name", name))
			return nil, fmt.Errorf("template not found: %s", name)
		}
		r.logger.Error("Error finding template by name", zap.String("name", name), zap.Error(err))
		return nil, fmt.Errorf("error finding template: %w", err)
	}

	return &t, nil
}

func (r *postgresTemplateRepository) FindByAction(ctx context.Context, action domain.NotificationAction, notificationType domain.NotificationType) (*domain.Template, error) {
	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, namespace, name, subject, file_path, action, type, version, is_active, created_at, updated_at
		FROM templates
		WHERE action = $1 AND type = $2 AND is_active = true
		ORDER BY version DESC
		LIMIT 1
	`

	var t domain.Template
	err = conn.QueryRowContext(ctx, query, action, notificationType).Scan(
		&t.ID, &t.Namespace, &t.Name, &t.Subject, &t.FilePath,
		&t.Action, &t.Type, &t.Version, &t.IsActive, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("Template not found for action",
				zap.String("action", string(action)), zap.String("type", string(notificationType)))
			return nil, fmt.Errorf("template not found for action: %s and type: %s", action, notificationType)
		}
		r.logger.Error("Error finding template by action",
			zap.String("action", string(action)), zap.String("type", string(notificationType)), zap.Error(err))
		return nil, fmt.Errorf("error finding template: %w", err)
	}

	return &t, nil
}

func (r *postgresTemplateRepository) Save(ctx context.Context, template *domain.Template) error {
	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	template.CreatedAt = now
	template.UpdatedAt = now

	query := `
		INSERT INTO templates (id, namespace, name, subject, file_path, action, type, version, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = conn.ExecContext(ctx, query,
		template.ID, template.Namespace, template.Name, template.Subject, template.FilePath,
		template.Action, template.Type, template.Version, template.IsActive,
		template.CreatedAt, template.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Error saving template", zap.String("id", template.ID), zap.Error(err))
		return fmt.Errorf("error saving template: %w", err)
	}

	r.logger.Info("Template saved successfully", zap.String("id", template.ID))
	return nil
}

func (r *postgresTemplateRepository) Update(ctx context.Context, template *domain.Template) error {
	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	template.UpdatedAt = time.Now()

	query := `
		UPDATE templates
		SET name = $2, subject = $3, file_path = $4, action = $5, type = $6, version = $7, is_active = $8, updated_at = $9
		WHERE id = $1
	`
	result, err := conn.ExecContext(ctx, query,
		template.ID, template.Name, template.Subject, template.FilePath,
		template.Action, template.Type, template.Version, template.IsActive, template.UpdatedAt,
	)
	if err != nil {
		r.logger.Error("Error updating template", zap.String("id", template.ID), zap.Error(err))
		return fmt.Errorf("error updating template: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("template not found: %s", template.ID)
	}

	r.logger.Info("Template updated successfully", zap.String("id", template.ID))
	return nil
}
