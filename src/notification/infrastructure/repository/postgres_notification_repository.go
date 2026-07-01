package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/database"
	"notifications/src/shared/logger"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// postgresNotificationRepository NO recibe un *sql.DB propio: opera siempre sobre la
// conexión fijada por database.TenantSession para el request (database.ConnFromContext),
// que ya trae namespace/tenant_id seteados como GUC de sesión. RLS (002_rls.sql) es la
// única fuente de aislamiento — ver docs/adr/ADR-001-rls-como-unica-fuente-de-aislamiento.md.
// Usar un *sql.DB del pool acá rompería el aislamiento en silencio.
type postgresNotificationRepository struct {
	logger *zap.Logger
}

func NewPostgresNotificationRepository() port.NotificationRepository {
	return &postgresNotificationRepository{
		logger: logger.GetLogger(),
	}
}

// nullIfEmpty mapea "" → NULL para columnas nullable (tenant_id, template_id, dedup_key).
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// connOrErr obtiene la conexión RLS-scoped del contexto. nil significa que se llamó fuera
// de un request HTTP con TenantSession montado (p. ej. un worker futuro) — fail-closed:
// nunca cae a una conexión del pool sin GUC seteadas.
func connOrErr(ctx context.Context) (*sql.Conn, error) {
	conn := database.ConnFromContext(ctx)
	if conn == nil {
		return nil, fmt.Errorf("no hay conexión RLS en el contexto (falta database.TenantSession)")
	}
	return conn, nil
}

// ExistsByDedupKey backstop de idempotencia en DB. Respaldo del nonce de Redis y del
// UNIQUE index parcial (namespace, COALESCE(tenant_id,''), dedup_key). dedupKey vacío → false.
func (r *postgresNotificationRepository) ExistsByDedupKey(ctx context.Context, dedupKey string) (bool, error) {
	if dedupKey == "" {
		return false, nil
	}
	conn, err := connOrErr(ctx)
	if err != nil {
		return false, err
	}

	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM notifications WHERE dedup_key = $1)`
	if err := conn.QueryRowContext(ctx, query, dedupKey).Scan(&exists); err != nil {
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

	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		r.logger.Error("Error marshaling notification data", zap.Error(err))
		return fmt.Errorf("error marshaling data: %w", err)
	}

	now := time.Now()
	notification.CreatedAt = now
	notification.UpdatedAt = now

	query := `
		INSERT INTO notifications (id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, dedup_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err = conn.ExecContext(ctx, query,
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
	if err != nil {
		r.logger.Error("Error saving notification", zap.String("id", notification.ID), zap.Error(err))
		return fmt.Errorf("error saving notification: %w", err)
	}

	r.logger.Info("Notification saved successfully", zap.String("id", notification.ID))
	return nil
}

func (r *postgresNotificationRepository) FindByID(ctx context.Context, id string) (*domain.Notification, error) {
	r.logger.Debug("Finding notification by ID", zap.String("id", id))

	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, created_at, updated_at
		FROM notifications
		WHERE id = $1
	`

	var n domain.Notification
	var dataJSON []byte
	var templateID sql.NullString
	var errorMessage sql.NullString
	var tenantID sql.NullString

	err = conn.QueryRowContext(ctx, query, id).Scan(
		&n.ID, &n.Namespace, &tenantID, &n.Type, &n.Action, &templateID,
		&n.Recipient, &dataJSON, &n.Status, &n.RetryCount, &errorMessage,
		&n.CreatedAt, &n.UpdatedAt,
	)
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

	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	dataJSON, err := json.Marshal(notification.Data)
	if err != nil {
		r.logger.Error("Error marshaling notification data", zap.Error(err))
		return fmt.Errorf("error marshaling data: %w", err)
	}

	notification.UpdatedAt = time.Now()

	query := `
		UPDATE notifications
		SET type = $2, action = $3, template_id = $4, recipient = $5, data = $6,
		    status = $7, retry_count = $8, error_message = $9, updated_at = $10
		WHERE id = $1
	`
	result, err := conn.ExecContext(ctx, query,
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
		r.logger.Error("Error updating notification", zap.String("id", notification.ID), zap.Error(err))
		return fmt.Errorf("error updating notification: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("notification not found: %s", notification.ID)
	}

	r.logger.Info("Notification updated successfully", zap.String("id", notification.ID))
	return nil
}

func (r *postgresNotificationRepository) UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus, errorMessage string) error {
	r.logger.Debug("Updating notification status", zap.String("id", id), zap.String("status", string(status)))

	conn, err := connOrErr(ctx)
	if err != nil {
		return err
	}

	query := `
		UPDATE notifications
		SET status = $2, error_message = $3, updated_at = $4
		WHERE id = $1
	`
	result, err := conn.ExecContext(ctx, query, id, status, errorMessage, time.Now())
	if err != nil {
		r.logger.Error("Error updating notification status", zap.String("id", id), zap.Error(err))
		return fmt.Errorf("error updating notification status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}

	r.logger.Info("Notification status updated successfully", zap.String("id", id), zap.String("status", string(status)))
	return nil
}

func (r *postgresNotificationRepository) FindPendingNotifications(ctx context.Context) ([]*domain.Notification, error) {
	r.logger.Debug("Finding pending notifications")

	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT id, namespace, tenant_id, type, action, template_id, recipient, data, status, retry_count, error_message, created_at, updated_at
		FROM notifications
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 100
	`
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		r.logger.Error("Error finding pending notifications", zap.Error(err))
		return nil, fmt.Errorf("error finding pending notifications: %w", err)
	}
	defer rows.Close()

	notifications, err := scanNotifications(rows, r.logger)
	if err != nil {
		return nil, err
	}

	r.logger.Info("Found pending notifications", zap.Int("count", len(notifications)))
	return notifications, nil
}

// FindByFilters busca notificaciones por filtros dinámicos. El scope namespace/tenant lo
// aplica RLS solo — ver ADR-001.
func (r *postgresNotificationRepository) FindByFilters(ctx context.Context, filters domain.NotificationFilters) ([]*domain.Notification, error) {
	r.logger.Debug("Finding notifications by filters", zap.Any("filters", filters))

	conn, err := connOrErr(ctx)
	if err != nil {
		return nil, err
	}

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

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		r.logger.Error("Error finding notifications by filters", zap.Error(err))
		return nil, fmt.Errorf("error finding notifications by filters: %w", err)
	}
	defer rows.Close()

	notifications, err := scanNotifications(rows, r.logger)
	if err != nil {
		return nil, err
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
