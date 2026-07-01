package appcontext

import (
	"context"
)

type rlsKey string

const (
	namespaceKey rlsKey = "rls.namespace"
	tenantKey    rlsKey = "rls.tenant_id"
)

// DefaultNamespace is the fallback namespace when none is provided.
const DefaultNamespace = "mc"

// WithRLS returns a new context with namespace and tenant_id for RLS scoping.
func WithRLS(ctx context.Context, namespace, tenantID string) context.Context {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	ctx = context.WithValue(ctx, namespaceKey, namespace)
	if tenantID != "" {
		ctx = context.WithValue(ctx, tenantKey, tenantID)
	}
	return ctx
}

// WithNamespace returns a new context with only namespace set.
func WithNamespace(ctx context.Context, namespace string) context.Context {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	return context.WithValue(ctx, namespaceKey, namespace)
}

// WithTenant returns a new context with tenant_id set.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey, tenantID)
}

// NamespaceFromContext extracts the RLS namespace from the context.
func NamespaceFromContext(ctx context.Context) string {
	if ns, ok := ctx.Value(namespaceKey).(string); ok && ns != "" {
		return ns
	}
	return DefaultNamespace
}

// TenantIDFromContext extracts the RLS tenant_id from the context.
func TenantIDFromContext(ctx context.Context) string {
	if tid, ok := ctx.Value(tenantKey).(string); ok {
		return tid
	}
	return ""
}
