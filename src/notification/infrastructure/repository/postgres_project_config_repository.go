package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// postgresProjectConfigRepository sigue el mismo patrón RLS-only que el resto: opera
// siempre sobre database.ConnFromContext, sin *sql.DB propio. project_config se filtra
// solo por namespace (004_project_config.sql, PK = namespace) — no hay tenant_id.
type postgresProjectConfigRepository struct {
	logger *zap.Logger
}

func NewPostgresProjectConfigRepository() port.ProjectConfigRepository {
	return &postgresProjectConfigRepository{
		logger: logger.GetLogger(),
	}
}

func (r *postgresProjectConfigRepository) FindByNamespace(ctx context.Context, namespace string) (*domain.ProjectConfig, error) {
	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT namespace, from_email, from_name, provider_creds_ref, channels_enabled, default_template_set, quota
		FROM project_config
		WHERE namespace = $1
	`

	var cfg domain.ProjectConfig
	var providerCredsRef sql.NullString
	var defaultTemplateSet sql.NullString
	var channelsJSON, quotaJSON []byte

	err = conn.QueryRowContext(ctx, query, namespace).Scan(
		&cfg.Namespace, &cfg.FromEmail, &cfg.FromName, &providerCredsRef,
		&channelsJSON, &defaultTemplateSet, &quotaJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("Project config not found", zap.String("namespace", namespace))
			return nil, fmt.Errorf("project config not found: %s", namespace)
		}
		r.logger.Error("Error finding project config", zap.String("namespace", namespace), zap.Error(err))
		return nil, fmt.Errorf("error finding project config: %w", err)
	}

	if providerCredsRef.Valid {
		cfg.ProviderCredsRef = providerCredsRef.String
	}
	if defaultTemplateSet.Valid {
		cfg.DefaultTemplateSet = defaultTemplateSet.String
	}
	if len(channelsJSON) > 0 {
		if err := json.Unmarshal(channelsJSON, &cfg.ChannelsEnabled); err != nil {
			r.logger.Error("Error unmarshaling channels_enabled", zap.Error(err))
			return nil, fmt.Errorf("error unmarshaling channels_enabled: %w", err)
		}
	}
	if len(quotaJSON) > 0 {
		if err := json.Unmarshal(quotaJSON, &cfg.Quota); err != nil {
			r.logger.Error("Error unmarshaling quota", zap.Error(err))
			return nil, fmt.Errorf("error unmarshaling quota: %w", err)
		}
	}

	return &cfg, nil
}

func (r *postgresProjectConfigRepository) Save(ctx context.Context, cfg *domain.ProjectConfig) error {
	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	channelsJSON, err := cfg.ChannelsEnabledJSON()
	if err != nil {
		return fmt.Errorf("error marshaling channels_enabled: %w", err)
	}
	quotaJSON, err := json.Marshal(cfg.Quota)
	if err != nil {
		return fmt.Errorf("error marshaling quota: %w", err)
	}

	query := `
		INSERT INTO project_config (namespace, from_email, from_name, provider_creds_ref, channels_enabled, default_template_set, quota)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err = conn.ExecContext(ctx, query,
		cfg.Namespace, cfg.FromEmail, cfg.FromName, nullIfEmpty(cfg.ProviderCredsRef),
		channelsJSON, nullIfEmpty(cfg.DefaultTemplateSet), quotaJSON,
	)
	if err != nil {
		r.logger.Error("Error saving project config", zap.String("namespace", cfg.Namespace), zap.Error(err))
		return fmt.Errorf("error saving project config: %w", err)
	}

	r.logger.Info("Project config saved successfully", zap.String("namespace", cfg.Namespace))
	return nil
}

func (r *postgresProjectConfigRepository) Update(ctx context.Context, cfg *domain.ProjectConfig) error {
	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	channelsJSON, err := cfg.ChannelsEnabledJSON()
	if err != nil {
		return fmt.Errorf("error marshaling channels_enabled: %w", err)
	}
	quotaJSON, err := json.Marshal(cfg.Quota)
	if err != nil {
		return fmt.Errorf("error marshaling quota: %w", err)
	}

	query := `
		UPDATE project_config
		SET from_email = $2, from_name = $3, provider_creds_ref = $4, channels_enabled = $5,
		    default_template_set = $6, quota = $7, updated_at = now()
		WHERE namespace = $1
	`
	result, err := conn.ExecContext(ctx, query,
		cfg.Namespace, cfg.FromEmail, cfg.FromName, nullIfEmpty(cfg.ProviderCredsRef),
		channelsJSON, nullIfEmpty(cfg.DefaultTemplateSet), quotaJSON,
	)
	if err != nil {
		r.logger.Error("Error updating project config", zap.String("namespace", cfg.Namespace), zap.Error(err))
		return fmt.Errorf("error updating project config: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("project config not found: %s", cfg.Namespace)
	}

	r.logger.Info("Project config updated successfully", zap.String("namespace", cfg.Namespace))
	return nil
}
