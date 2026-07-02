package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	appctx "notifications/src/notification/application/appcontext"
	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/logger"

	"github.com/hornosg/go-shared/infrastructure/postgres"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// postgresNotificationRepository NO recibe una conexión propia por request: cada método
// abre su propia transacción vía go-shared postgres.WithRLSInTransaction, que fija
// app.tenant_id/app.namespace con SET LOCAL antes de la query — se resetea solo al
// COMMIT/ROLLBACK, sin conexión pooleada que fijar/soltar a mano (ver PROP-007, que
// reemplaza la implementación local anterior basada en database.ConnFromContext, PROP-007).
// RLS (002_rls.sql) es la única fuente de aislamiento — ver ADR-001.
type postgresNotificationRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

func NewPostgresNotificationRepository(db *sql.DB) port.NotificationRepository {
	return &postgresNotificationRepository{
		db:     db,
		logger: logger.GetLogger(),
	}
}

// rlsContext arma el RLSContext de go-shared desde el appctx del request (namespace/tenant
// ya resueltos por middleware.ContextWithRLSFromGin, en última instancia desde el JWT).
func rlsContext(ctx context.Context) postgres.RLSContext {
	return postgres.RLSContext{
		TenantID:  appctx.TenantIDFromContext(ctx),
		Namespace: appctx.NamespaceFromContext(ctx),
	}
}

// nullIfEmpty mapea "" → NULL para columnas nullable (tenant_id, template_id, dedup_key).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ExistsByDedupKey backstop de idempotencia en DB. Respaldo del nonce de Redis y del
// UNIQUE index parcial (namespace, COALESCE(tenant_id,''), dedup_key). dedupKey vacío → false.
func (r *postgresNotificationRepository) ExistsByDedupKey(ctx context.Context, dedupKey string) (bool, error) {
	if dedupKey == "" {
		return false, nil
	}

	var exists bool
	err := postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `SELECT EXISTS (SELECT 1 FROM notifications WHERE dedup_key = $1)`
		return tx.QueryRowContext(ctx, query, dedupKey).Scan(&exists)
	})
	if err != nil {
		r.logger.Error("Error checking dedup key existence", zap.String("dedup_key", dedupKey), zap.Error(err))
		return false, fmt.Errorf("error checking dedup key: %w", err)
	}
	return exists, nil
}

func (r *postgresNotificationRepository) Save(ctx context.Context, notification *domain.Notification) error {
	r.logger.Debug("Saving notification to PostgreSQL",
		zap.String("id", notification.ID),
		zap.String("type", string(notification.Type)),
		zap.String("action", string(notification.Action)))

	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		r.logger.Error("Error marshaling notification data", zap.Error(err))
		return fmt.Errorf("error marshaling data: %w", err)
	}

	now := time.Now()
	notification.CreatedAt = now
	notification.UpdatedAt = now

	err = postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `
			INSERT INTO notifications (id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, dedup_key, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		`
		_, err := tx.ExecContext(ctx, query,
			notification.ID,
			notification.Namespace,
			nullIfEmpty(notification.TenantID),
			notification.Type,
			notification.Action,
			nullIfEmpty(notification.TemplateID),
			notification.Recipient,
			dataJSON,
			notification.Status,
			notification.RetryCount,
			notification.Error,
			nullIfEmpty(notification.DedupKey),
			notification.CreatedAt,
			notification.UpdatedAt,
		)
		return err
	})
	if err != nil {
		r.logger.Error("Error saving notification", zap.String("id", notification.ID), zap.Error(err))
		return fmt.Errorf("error saving notification: %w", err)
	}

	r.logger.Info("Notification saved successfully", zap.String("id", notification.ID))
	return nil
}

func (r *postgresNotificationRepository) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	r.logger.Debug("Finding notification by ID", zap.String("id", id))

	var n domain.Notification
	var dataJSON []byte
	var templateID sql.NullString
	var errorMessage sql.NullString
	var tenantID sql.NullString

	err := postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `
			SELECT id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, created_at, updated_at
			FROM notifications
			WHERE id = $1
		`
		return tx.QueryRowContext(ctx, query, id).Scan(
			&n.ID, &n.Namespace, &tenantID, &n.Type, &n.Action, &templateID,
			&n.Recipient, &dataJSON, &n.Status, &n.RetryCount, &errorMessage,
			&n.CreatedAt, &n.UpdatedAt,
		)
	})
	if err != nil {
		if err == sql.ErrNoRows {
			r.logger.Warn("Notification not found", zap.String("id", id))
			return nil, fmt.Errorf("notification not found: %s", id)
		}
		r.logger.Error("Error finding notification by ID", zap.String("id", id), zap.Error(err))
		return nil, fmt.Errorf("error finding notification: %w", err)
	}

	if len(dataJSON) > 0 {
		if err := json.Unmarshal(dataJSON, &n.Data); err != nil {
			r.logger.Error("Error unmarshaling notification data", zap.Error(err))
			return nil, fmt.Errorf("error unmarshaling data: %w", err)
		}
	}
	if templateID.Valid {
		n.TemplateID = templateID.String
	}
	if errorMessage.Valid {
		n.Error = errorMessage.String
	}
	if tenantID.Valid {
		n.TenantID = tenantID.String
	}

	r.logger.Debug("Notification found", zap.String("id", id))
	return &n, nil
}

func (r *postgresNotificationRepository) Update(ctx context.Context, notification *domain.Notification) error {
	r.logger.Debug("Updating notification", zap.String("id", notification.ID))

	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		r.logger.Error("Error marshaling notification data", zap.Error(err))
		return fmt.Errorf("error marshaling data: %w", err)
	}

	notification.UpdatedAt = time.Now()

	var rowsAffected int64
	err = postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `
			UPDATE notifications
			SET type = $2, action = $3, template_id = $4, recipient = $5, data = $6,
			    status = $7, retry_count = $8, error_message = $9, updated_at = $10
			WHERE id = $1
		`
		result, err := tx.ExecContext(ctx, query,
			notification.ID,
			notification.Type,
			notification.Action,
			nullIfEmpty(notification.TemplateID),
			notification.Recipient,
			dataJSON,
			notification.Status,
			notification.RetryCount,
			notification.Error,
			notification.UpdatedAt,
		)
		if err != nil {
			return err
		}
		rowsAffected, _ = result.RowsAffected()
		return nil
	})
	if err != nil {
		r.logger.Error("Error updating notification", zap.String("id", notification.ID), zap.Error(err))
		return fmt.Errorf("error updating notification: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("notification not found: %s", notification.ID)
	}

	r.logger.Info("Notification updated successfully", zap.String("id", notification.ID))
	return nil
}

func (r *postgresNotificationRepository) UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus, errorMessage string) error {
	r.logger.Debug("Updating notification status", zap.String("id", id), zap.String("status", string(status)))

	var rowsAffected int64
	err := postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `
			UPDATE notifications
			SET status = $2, error_message = $3, updated_at = $4
			WHERE id = $1
		`
		result, err := tx.ExecContext(ctx, query, id, status, errorMessage, time.Now())
		if err != nil {
			return err
		}
		rowsAffected, _ = result.RowsAffected()
		return nil
	})
	if err != nil {
		r.logger.Error("Error updating notification status", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("error updating notification status: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}

	r.logger.Info("Notification status updated successfully", zap.String("id", id), zap.String("status", string(status)))
	return nil
}

func (r *postgresNotificationRepository) FindPendingNotifications(ctx context.Context) ([]*domain.Notification, error) {
	r.logger.Debug("Finding pending notifications")

	var notifications []*domain.Notification
	err := postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `
			SELECT id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, created_at, updated_at
			FROM notifications
			WHERE status = 'pending'
			ORDER BY created_at ASC
			LIMIT 100
		`
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()

		notifications, err = scanNotifications(rows, r.logger)
		return err
	})
	if err != nil {
		r.logger.Error("Error finding pending notifications", zap.Error(err))
		return nil, fmt.Errorf("error finding pending notifications: %w", err)
	}

	r.logger.Info("Found pending notifications", zap.Int("count", len(notifications)))
	return notifications, nil
}

// FindByFilters busca notificaciones por filtros dinámicos. El scope namespace/tenant lo
// aplica RLS solo — ver ADR-001.
func (r *postgresNotificationRepository) FindByFilters(ctx context.Context, filters domain.NotificationFilters) ([]*domain.Notification, error) {
	r.logger.Debug("Finding notifications by filters", zap.Any("filters", filters))

	var notifications []*domain.Notification
	err := postgres.WithRLSInTransaction(ctx, r.db, rlsContext(ctx), func(ctx context.Context, tx *sql.Tx) error {
		query := `
			SELECT id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, created_at, updated_at
			FROM notifications
			WHERE 1 = 1
		`
		var args []interface{}
		argIndex := 1

		if filters.Type != nil {
			query += fmt.Sprintf(" AND type = $%d", argIndex)
			args = append(args, *filters.Type)
			argIndex++
		}
		if filters.Action != nil {
			query += fmt.Sprintf(" AND action = $%d", argIndex)
			args = append(args, *filters.Action)
			argIndex++
		}
		if filters.Recipient != nil {
			query += fmt.Sprintf(" AND recipient = $%d", argIndex)
			args = append(args, *filters.Recipient)
			argIndex++
		}
		if filters.Status != nil {
			query += fmt.Sprintf(" AND status = $%d", argIndex)
			args = append(args, *filters.Status)
			argIndex++
		}

		query += " ORDER BY created_at DESC"

		limit := filters.Limit
		if limit <= 0 {
			limit = 50
		}
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
		argIndex++

		if filters.Offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argIndex)
			args = append(args, filters.Offset)
		}

		r.logger.Debug("Executing query", zap.String("query", query), zap.Any("args", args))

		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		notifications, err = scanNotifications(rows, r.logger)
		return err
	})
	if err != nil {
		r.logger.Error("Error finding notifications by filters", zap.Error(err))
		return nil, fmt.Errorf("error finding notifications by filters: %w", err)
	}

	r.logger.Info("Found notifications by filters", zap.Int("count", len(notifications)), zap.Any("filters", filters))
	return notifications, nil
}

// scanNotifications centraliza el scan de filas repetido en FindPendingNotifications y
// FindByFilters (mismo SELECT, misma forma de fila).
func scanNotifications(rows *sql.Rows, log *zap.Logger) ([]*domain.Notification, error) {
	var notifications []*domain.Notification

	for rows.Next() {
		var n domain.Notification
		var dataJSON []byte
		var templateID sql.NullString
		var errorMessage sql.NullString
		var tenantID sql.NullString

		if err := rows.Scan(
			&n.ID, &n.Namespace, &tenantID, &n.Type, &n.Action, &templateID,
			&n.Recipient, &dataJSON, &n.Status, &n.RetryCount, &errorMessage,
			&n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			log.Error("Error scanning notification row", zap.Error(err))
			return nil, fmt.Errorf("error scanning notification: %w", err)
		}

		if len(dataJSON) > 0 {
			if err := json.Unmarshal(dataJSON, &n.Data); err != nil {
				log.Error("Error unmarshaling notification data", zap.Error(err))
				continue
			}
		}
		if templateID.Valid {
			n.TemplateID = templateID.String
		}
		if errorMessage.Valid {
			n.Error = errorMessage.String
		}
		if tenantID.Valid {
			n.TenantID = tenantID.String
		}

		notifications = append(notifications, &n)
	}
	if err := rows.Err(); err != nil {
		log.Error("Error iterating notification rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating notifications: %w", err)
	}

	return notifications, nil
}
