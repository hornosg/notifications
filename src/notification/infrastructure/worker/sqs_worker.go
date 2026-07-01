package worker

import (
	"context"
	"database/sql"
	"time"

	appctx "notifications/src/notification/application/appcontext"
	"notifications/src/notification/domain"
	"notifications/src/notification/domain/port"
	"notifications/src/shared/database"
	"notifications/src/shared/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SQSWorker corre FUERA del ciclo HTTP: database.TenantSession (el middleware Gin que fija
// la conexión + GUC de sesión por request) nunca se ejecuta acá. Por eso el worker recibe
// el *sql.DB del pool y usa database.WithScopedConn para fijar su PROPIA conexión + GUCs
// antes de tocar cualquier repository — mismo fail-closed que un request HTTP, aplicado
// manualmente (advertido en ADR-001 al portar E23).
type SQSWorker struct {
	queue            port.Queue
	emailSender      port.EmailSender
	notificationRepo port.NotificationRepository
	db               *sql.DB
	logger           *zap.Logger
	stopChan         chan struct{}
	running          bool
}

func NewSQSWorker(
	queue port.Queue,
	emailSender port.EmailSender,
	notificationRepo port.NotificationRepository,
	db *sql.DB,
) *SQSWorker {
	return &SQSWorker{
		queue:            queue,
		emailSender:      emailSender,
		notificationRepo: notificationRepo,
		db:               db,
		logger:           logger.GetLogger(),
		stopChan:         make(chan struct{}),
		running:          false,
	}
}

func (w *SQSWorker) Start(ctx context.Context) {
	if w.running {
		w.logger.Warn("SQS Worker is already running")
		return
	}
	w.running = true
	w.logger.Info("Starting SQS Worker")
	go w.processMessages(ctx)
}

func (w *SQSWorker) Stop() {
	if !w.running {
		w.logger.Warn("SQS Worker is not running")
		return
	}
	w.logger.Info("Stopping SQS Worker")
	close(w.stopChan)
	w.running = false
}

func (w *SQSWorker) processMessages(ctx context.Context) {
	w.logger.Info("SQS Worker started and listening for messages")
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("SQS Worker stopped due to context cancellation")
			return
		case <-w.stopChan:
			w.logger.Info("SQS Worker stopped")
			return
		default:
			w.processNextMessage(ctx)
		}
	}
}

func (w *SQSWorker) ProcessNext(ctx context.Context) {
	dequeueCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	notification, err := w.queue.Dequeue(dequeueCtx)
	if err != nil {
		w.logger.Error("Failed to dequeue message from SQS", zap.Error(err))
		time.Sleep(5 * time.Second)
		return
	}
	if notification == nil {
		w.logger.Debug("No messages available in queue")
		return
	}

	w.processNotification(ctx, notification)
}

func (w *SQSWorker) processNotification(ctx context.Context, notification *domain.Notification) {
	if notification.Namespace == "" {
		notification.Namespace = appctx.DefaultNamespace
	}
	if notification.ID == "" {
		notification.ID = uuid.New().String()
		w.logger.Warn("No notification_id in SQS message, generating new one",
			zap.String("generated_id", notification.ID))
	}

	w.logger.Info("Processing notification from SQS",
		zap.String("notification_id", notification.ID),
		zap.String("namespace", notification.Namespace),
		zap.String("tenant_id", notification.TenantID),
		zap.String("action", string(notification.Action)),
		zap.String("recipient", notification.Recipient))

	if w.db == nil {
		w.logger.Error("No DB pool configured for worker, cannot process notification (fail-closed)",
			zap.String("notification_id", notification.ID))
		return
	}

	err := database.WithScopedConn(ctx, w.db, notification.Namespace, notification.TenantID, w.logger, func(scopedCtx context.Context) error {
		// appctx.WithRLS deja disponible namespace/tenant para el emailSender (resolución
		// de identidad de envío por proyecto), en paralelo a la conexión ya scoped por RLS.
		scopedCtx = appctx.WithRLS(scopedCtx, notification.Namespace, notification.TenantID)
		w.handleScoped(scopedCtx, notification)
		return nil
	})
	if err != nil {
		w.logger.Error("Failed to fix scoped DB connection for worker",
			zap.String("notification_id", notification.ID), zap.Error(err))
	}
}

func (w *SQSWorker) handleScoped(scopedCtx context.Context, notification *domain.Notification) {
	if w.notificationRepo != nil {
		notification.Status = domain.StatusPending
		if err := w.notificationRepo.Update(scopedCtx, notification); err != nil {
			w.logger.Error("Failed to update notification status to processing",
				zap.String("notification_id", notification.ID), zap.Error(err))
		}
	}

	if err := w.sendNotification(scopedCtx, notification); err != nil {
		w.logger.Error("Failed to send notification",
			zap.String("notification_id", notification.ID), zap.String("action", string(notification.Action)), zap.Error(err))

		notification.Status = domain.StatusFailed
		notification.Error = err.Error()
		if w.notificationRepo != nil {
			if updateErr := w.notificationRepo.Update(scopedCtx, notification); updateErr != nil {
				w.logger.Error("Failed to update notification status to failed",
					zap.String("notification_id", notification.ID), zap.Error(updateErr))
			}
		}
		return
	}

	w.logger.Info("Notification sent successfully",
		zap.String("notification_id", notification.ID), zap.String("action", string(notification.Action)))

	notification.Status = domain.StatusSent
	notification.Error = ""
	if w.notificationRepo != nil {
		if err := w.notificationRepo.Update(scopedCtx, notification); err != nil {
			w.logger.Error("Failed to update notification status to sent",
				zap.String("notification_id", notification.ID), zap.Error(err))
		}
	}
}

func (w *SQSWorker) processNextMessage(ctx context.Context) {
	w.ProcessNext(ctx)
}

func (w *SQSWorker) sendNotification(ctx context.Context, notification *domain.Notification) error {
	switch notification.Type {
	case domain.EmailNotification:
		return w.emailSender.SendEmailByAction(ctx, notification.Recipient, notification.Action, notification.Type, notification.Data)
	default:
		w.logger.Warn("Unknown notification type", zap.String("type", string(notification.Type)))
		return nil
	}
}

func (w *SQSWorker) IsRunning() bool {
	return w.running
}

func (w *SQSWorker) GetQueueSize(ctx context.Context) (int64, error) {
	return w.queue.Size(ctx)
}
