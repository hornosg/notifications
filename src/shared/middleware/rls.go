// Package middleware — helpers Gin que arman el contexto de aplicación (appctx.WithRLS)
// para los use cases a partir del request ya autenticado.
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"

	appctx "notifications/src/notification/application/appcontext"
	"notifications/src/shared/database"
)

// ContextWithRLSFromGin arma el context.Context que consumen los use cases: namespace
// SIEMPRE del claim JWT ya validado (database.NamespaceFromClaims — nunca de un header
// crudo, ese fue el CRITICAL original de notification-service que motivó E23/E07) y
// tenant_id del header X-Tenant-ID (mismo criterio que database.TenantSession, que ya
// corrió antes en la cadena de middlewares y fijó la conexión RLS-scoped).
func ContextWithRLSFromGin(c *gin.Context) context.Context {
	namespace, _ := database.NamespaceFromClaims(c)
	tenantID := c.GetHeader("X-Tenant-ID")
	return appctx.WithRLS(c.Request.Context(), namespace, tenantID)
}
