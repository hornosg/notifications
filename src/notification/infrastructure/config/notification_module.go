package config

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"notifications/pkg/validator"
	"notifications/src/notification/application/usecase"
	"notifications/src/notification/domain/port"
	"notifications/src/notification/infrastructure/controller"
	"notifications/src/notification/infrastructure/dedup"
	"notifications/src/notification/infrastructure/email"
	notificationevent "notifications/src/notification/infrastructure/event"
	notificationlog "notifications/src/notification/infrastructure/logging"
	"notifications/src/notification/infrastructure/queue"
	"notifications/src/notification/infrastructure/repository"
	"notifications/src/notification/infrastructure/template"
	"notifications/src/notification/infrastructure/worker"
	sharedconfig "notifications/src/shared/config"
	"notifications/src/shared/logger"

	"github.com/gin-gonic/gin"
	"github.com/mercadocercano/eventbus"
	"github.com/redis/go-redis/v9"
)

// SetupNotificationModule cablea repositories, servicios y handlers del bounded context.
// db puede ser nil (arranque sin DB — health/metrics OK, ver main.go connectDB): en ese
// caso los repositories quedan nil y las rutas devuelven error de servicio no disponible,
// nunca silencian el fallo con datos falsos.
func SetupNotificationModule(router *gin.RouterGroup, cfg *sharedconfig.Config, db *sql.DB) {
	var templateRepo port.TemplateRepository
	var notificationRepo port.NotificationRepository
	var projectConfigRepo port.ProjectConfigRepository

	if db != nil {
		templateRepo = repository.NewPostgresTemplateRepository()
		notificationRepo = repository.NewPostgresNotificationRepository()
		projectConfigRepo = repository.NewPostgresProjectConfigRepository()
	}

	templateService := template.NewTemplateService(templateRepo, cfg.Contact.Email)

	var emailSender port.EmailSender
	if cfg.Resend.MockSender {
		emailSender = email.NewMockSender()
		log.Printf("Using mock email sender (no real emails will be sent)")
	} else {
		emailSender = email.NewResendClient(cfg.Resend.APIKey, cfg.Resend.FromEmail, templateService, projectConfigRepo)
	}
	emailValidator := validator.NewEmailValidator()

	var queueService port.Queue
	var sqsWorker *worker.SQSWorker

	if cfg.SQS.Enabled {
		log.Printf("Initializing SQS queue with URL: %s, Region: %s", cfg.SQS.QueueURL, cfg.SQS.Region)

		sqsQueue, err := queue.NewSQSQueue(queue.SQSConfig{QueueURL: cfg.SQS.QueueURL, Region: cfg.SQS.Region})
		if err != nil {
			log.Printf("Warning: Could not initialize SQS queue: %v", err)
		} else {
			queueService = sqsQueue
			log.Printf("SQS queue initialized successfully")

			sqsWorker = worker.NewSQSWorker(queueService, emailSender, notificationRepo, db)
			sqsWorker.Start(context.Background())
			log.Printf("SQS worker started successfully")
		}
	} else {
		log.Printf("SQS is disabled, async notifications will not work")
	}

	eventLogger := notificationlog.NewNotificationLogger("notifications")

	var deduplicator port.Deduplicator
	if cfg.Redis.Host != "" {
		redisClient := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		deduplicator = dedup.NewRedisDeduplicator(redisClient, time.Hour)
		log.Printf("Redis deduplicator initialized (%s:%d)", cfg.Redis.Host, cfg.Redis.Port)
	} else {
		log.Printf("REDIS_HOST not set, dedup falls back to DB backstop only")
	}

	sendNotificationUseCase := usecase.NewSendNotificationUseCase(
		notificationRepo, templateRepo, emailSender, queueService, emailValidator,
	).WithEventLogger(eventLogger).WithDeduplicator(deduplicator)

	getNotificationUseCase := usecase.NewGetNotificationUseCase(notificationRepo)
	listNotificationsUseCase := usecase.NewListNotificationsUseCase(notificationRepo)

	notificationHandler := controller.NewNotificationHandler(
		sendNotificationUseCase, getNotificationUseCase, listNotificationsUseCase,
	)
	notificationHandler.RegisterRoutes(router)

	if cfg.SQS.Enabled && sqsWorker != nil {
		setupQueueMonitoringRoutes(router, sqsWorker)
	}

	// Worker consumer del EventBus. Opt-in por EVENTBUS_ENABLED para no colgar el pod antes
	// de cablear los secrets EVENTBUS_DB_*. Best-effort: si falla, loguea y sigue — el path
	// HTTP sync no depende del EventBus.
	setupEventWorker(cfg, sendNotificationUseCase, eventLogger, db)
}

func setupEventWorker(cfg *sharedconfig.Config, sender notificationevent.NotificationSender, eventLogger *notificationlog.NotificationLogger, db *sql.DB) {
	if !cfg.EventBus.Enabled {
		log.Printf("EventBus consumer disabled (EVENTBUS_ENABLED != true), event-driven notifications inactive")
		return
	}
	if db == nil {
		log.Printf("Warning: no DB pool available, EventBus consumer disabled (fail-closed)")
		return
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.EventBus.Host, cfg.EventBus.Port, cfg.EventBus.User, cfg.EventBus.Password, cfg.EventBus.Name, cfg.EventBus.SSLMode,
	)
	eventbusDB, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Printf("Warning: could not open EventBus DB connection, event-driven notifications disabled: %v", err)
		return
	}
	if err := eventbusDB.Ping(); err != nil {
		log.Printf("Warning: could not connect to EventBus DB, event-driven notifications disabled: %v", err)
		return
	}

	infraLogger := eventbus.NewLogger(eventbus.LevelInfo)
	eventStore := eventbus.NewSQLEventStore(eventbusDB, infraLogger)
	processUseCase := eventbus.NewProcessEventUseCase(eventStore, infraLogger)

	eventWorker := eventbus.NewEventWorker(processUseCase, infraLogger, 10, 5*time.Second)

	handler := notificationevent.NewNotificationEventHandler(sender, eventLogger, db, logger.GetLogger())
	if err := eventWorker.RegisterHandler(handler); err != nil {
		log.Printf("Warning: could not register notification event handler: %v", err)
		return
	}

	if err := eventWorker.Start(context.Background()); err != nil {
		log.Printf("Warning: could not start EventBus worker: %v", err)
		return
	}
	log.Printf("EventBus consumer started (consumer=%s, batch=10, poll=5s)", handler.ConsumerName())
}

func setupQueueMonitoringRoutes(router *gin.RouterGroup, sqsWorker *worker.SQSWorker) {
	router.GET("/queue/status", func(c *gin.Context) {
		ctx := c.Request.Context()

		size, err := sqsWorker.GetQueueSize(ctx)
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to get queue size", "details": err.Error()})
			return
		}

		c.JSON(200, gin.H{"queue_size": size, "worker_running": sqsWorker.IsRunning()})
	})
}
