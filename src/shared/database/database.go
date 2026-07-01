// Package database — conexión a Postgres y sesión de tenant fail-closed (Devy RULE-10).
package database

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

const connKey = "tenant_db_conn"

// Connect abre el pool contra lab-postgres usando el rol de app (sin DDL, RULE-09).
func Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		env("DATABASE_HOST", "lab-postgres"), env("DATABASE_PORT", "5432"),
		env("DATABASE_USER", "notifications_app"), os.Getenv("DATABASE_PASSWORD"),
		env("DATABASE_NAME", "notifications"), env("DATABASE_SSL_MODE", "disable"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

// TenantSession fija UNA conexión por request y setea app.tenant_id + app.namespace en ESA
// conexión, de modo que las policies RLS (002_rls.sql) apliquen aunque el handler olvide
// filtrar. El tenant sale del header X-Tenant-ID (ya validado contra el JWT por go-shared).
//
// El namespace (proyecto/IDP — notifications es cross-project, no solo multi-tenant dentro
// de un proyecto) NUNCA sale de un header crudo: se lee del claim "namespace" del JWT ya
// validado por su firma (c.Get("jwt_claims")). Un X-Namespace de header sin firmar es
// spoofeable — ese fue el CRITICAL original que motivó el audit de E07/E23 sobre
// notification-service (namespace resuelto de header no autenticado). Fail-closed: sin
// namespace claim, se rechaza el request (no hay default silencioso a "mc" a este nivel).
//
// Debe montarse DESPUÉS de tenantmw.TenantValidation: depende de que "roles" y
// "jwt_claims" ya estén en el contexto (Devy RULE-10, sesión L4 de E07 2026-07-01).
func TenantSession(db *sql.DB, log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant := c.GetHeader("X-Tenant-ID")
		isSystemAdmin := hasRole(c, "system_admin")
		if tenant == "" && !isSystemAdmin {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "missing X-Tenant-ID"})
			return
		}

		namespace, ok := namespaceFromValidatedClaims(c)
		if !ok && !isSystemAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing namespace claim"})
			return
		}

		conn, err := db.Conn(c.Request.Context())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "db unavailable"})
			return
		}
		// Orden de defers (LIFO): primero se resetean las GUC de sesión, recién
		// después se devuelve la conexión al pool. Sin este reset, app.tenant_id /
		// app.namespace / app.role quedan "pegados" a la conexión física y contaminan
		// al próximo request que la recicle — CRITICAL-2 detectado por @dev-security
		// en la sesión L4 de E07 (2026-07-01).
		defer conn.Close()
		defer resetTenantSession(c.Request.Context(), conn, log)

		// set_config(..., false) = a nivel de sesión de ESTA conexión fijada.
		if _, err := conn.ExecContext(c.Request.Context(),
			"SELECT set_config('app.tenant_id', $1, false)", tenant); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "tenant session failed"})
			return
		}
		if namespace != "" {
			if _, err := conn.ExecContext(c.Request.Context(),
				"SELECT set_config('app.namespace', $1, false)", namespace); err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "namespace session failed"})
				return
			}
		}

		// Break-glass del owner: el rol sale de los claims JWT YA VALIDADOS por
		// tenantmw.TenantValidation, nunca del header X-User-Role crudo — un
		// request no autenticado como system_admin no puede autoasignarse el rol
		// cross-tenant. CRITICAL-1 detectado por @dev-security en la sesión L4 de
		// E07 (2026-07-01). Queda auditado con el actor concreto (Ley 25.326).
		if isSystemAdmin {
			if _, err := conn.ExecContext(c.Request.Context(),
				"SELECT set_config('app.role', 'system_admin', false)"); err == nil {
				log.Warn("break_glass_access",
					zap.String("event", "system_admin_cross_tenant"),
					zap.String("user_id", userIDFromContext(c)),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					zap.String("tenant_hint", tenant),
				)
			}
		}

		c.Set(connKey, conn)
		c.Next()
	}
}

// resetTenantSession limpia las GUC de sesión (app.tenant_id, app.namespace, app.role)
// antes de que la conexión vuelva al pool. Postgres no resetea automáticamente las
// session vars al liberar una conexión pooleada — si no se limpian acá, el próximo
// request que recicle esta conexión física hereda el tenant/namespace/rol del anterior.
func resetTenantSession(ctx context.Context, conn *sql.Conn, log *zap.Logger) {
	if _, err := conn.ExecContext(ctx, "RESET app.tenant_id; RESET app.namespace; RESET app.role"); err != nil {
		log.Error("no pude resetear la sesión de tenant antes de devolver la conexión al pool",
			zap.Error(err))
	}
}

// namespaceFromValidatedClaims extrae el claim "namespace" de los claims JWT ya validados
// por tenantmw.TenantValidation (firma verificada). Nunca lee de un header — ver comentario
// de TenantSession. ok=false cuando no hay claim (p. ej. bypass histórico de tenant sin
// claims, o token S2S sin namespace).
func namespaceFromValidatedClaims(c *gin.Context) (string, bool) {
	v, exists := c.Get("jwt_claims")
	if !exists {
		return "", false
	}
	claims, ok := v.(jwt.MapClaims)
	if !ok {
		return "", false
	}
	ns, ok := claims["namespace"].(string)
	if !ok || ns == "" {
		return "", false
	}
	return ns, true
}

// hasRole verifica el rol contra los claims JWT ya validados por tenantmw.TenantValidation
// (disponibles en el contexto bajo la key "roles"). Fail-closed: si no hay roles en
// contexto (p. ej. bypass histórico de tenant sin claims), nunca concede el rol pedido.
func hasRole(c *gin.Context, role string) bool {
	v, exists := c.Get("roles")
	if !exists {
		return false
	}
	roles, ok := v.([]string)
	if !ok {
		return false
	}
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}

// userIDFromContext extrae el user_id de los claims JWT ya validados, para que el audit
// trail de break-glass identifique al actor concreto y no solo tenant/path (Ley 25.326).
func userIDFromContext(c *gin.Context) string {
	v, exists := c.Get("jwt_claims")
	if !exists {
		return "unknown"
	}
	claims, ok := v.(jwt.MapClaims)
	if !ok {
		return "unknown"
	}
	if uid, ok := claims["user_id"].(string); ok && uid != "" {
		return uid
	}
	return "unknown"
}

// Conn devuelve la conexión fijada del request. Los handlers la usan para sus queries.
func Conn(c *gin.Context) *sql.Conn {
	if v, ok := c.Get(connKey); ok {
		if conn, ok := v.(*sql.Conn); ok {
			return conn
		}
	}
	return nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
