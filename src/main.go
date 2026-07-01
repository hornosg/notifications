package main

// notifications — Gateway de notificaciones del lab (email/SMS) — recreación de notification-service, aislamiento fail-closed corregido (E07/E23)
// Servicio Go de la plataforma Devy. Hexagonal; este main es el composition root.

import (
	"database/sql"
	"os"

	"github.com/gin-gonic/gin"
	tenantmw "github.com/hornosg/go-shared/infrastructure/middleware"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"notifications/src/shared/database"
)

func main() {
	_ = godotenv.Load()

	log := newLogger()
	defer func() { _ = log.Sync() }()

	multitenant := env("MULTITENANT", "true") == "true"
	port := env("SERVER_PORT", "8282")
	if env("SERVER_MODE", "debug") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(cors())

	// Health y métricas: fuera del tenant middleware.
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "up", "service": "notifications"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Identidad/tenancy de la plataforma (go-shared valida el JWT emitido por IAM).
	r.Use(tenantmw.TenantValidation(tenantmw.TenantValidationConfig{
		JWTSecret:           os.Getenv("JWT_SECRET"),
		ExcludedRoutes:      []string{"/health", "/metrics"},
		RejectMissingTenant: multitenant, // fail-closed a nivel HTTP
	}))

	// Aislamiento fail-closed a nivel DB (RULE-10): conexión fijada + SET app.tenant_id.
	db := connectDB(log)
	if db != nil && multitenant {
		r.Use(database.TenantSession(db, log))
	}

	// ── Rutas del dominio (montar acá los handlers de src/notifications/...) ──
	api := r.Group("/api/v1")
	api.GET("/example", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true, "tenant_scoped": multitenant})
	})

	log.Info("starting", zap.String("service", "notifications"), zap.String("port", port))
	if err := r.Run(":" + port); err != nil {
		log.Fatal("server stopped", zap.Error(err))
	}
}

func connectDB(log *zap.Logger) *sql.DB {
	if os.Getenv("DATABASE_PASSWORD") == "" {
		log.Warn("DATABASE_PASSWORD vacío — arranco sin DB (health/metrics OK)")
		return nil
	}
	db, err := database.Connect()
	if err != nil {
		log.Warn("no pude conectar a la DB — sigo sin tenant session", zap.Error(err))
		return nil
	}
	log.Info("db connected")
	return db
}

// newLogger — envelope alineado a ADR-001 (Canonical Logs de mercado-cercano, adoptado
// como convención de plataforma): ts (RFC3339 UTC) / level / service / event, JSON a stdout.
// Este logger de arranque/infra usa zap (interno del proceso, no domain events); para
// eventos de dominio del servicio, definir puerto + struct por bounded context y delegar
// en go-shared/infrastructure/logging.CanonicalLogger (ver skill canonical-logs-go), NUNCA
// loguear domain events directo con zap.
func newLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.MessageKey = "event"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	l, err := cfg.Build(zap.Fields(zap.String("service", "notifications")))
	if err != nil {
		return zap.NewNop()
	}
	return l
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Tenant-ID, X-User-Role")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
