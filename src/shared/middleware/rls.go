// Package middleware — helpers Gin que arman el contexto de aplicación (appctx.WithRLS)
// para los use cases a partir del request ya autenticado.
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	tenantmw "github.com/hornosg/go-shared/infrastructure/middleware"

	appctx "notifications/src/notification/application/appcontext"
)

// ContextWithRLSFromGin arma el context.Context que consumen los use cases: namespace
// SIEMPRE del claim JWT ya validado — tenantmw.NamespaceFromContext (go-shared v0.15.0,
// PROP-007), que TenantValidation deja seteado en el gin.Context tras verificar la firma.
// Nunca de un header crudo: ese fue el CRITICAL original de notification-service que
// motivó E23/E07. tenant_id sale de "tenant_id" en el gin.Context, ya cruzado por
// TenantValidation contra el header X-Tenant-ID (mismatch = 403 antes de llegar acá).
func ContextWithRLSFromGin(c *gin.Context) context.Context {
	namespace, _ := tenantmw.NamespaceFromContext(c)
	tenantID := c.GetString("tenant_id")
	return appctx.WithRLS(c.Request.Context(), namespace, tenantID)
}
