package tenant

import "context"

// contextKey is a private type for tenant context keys to prevent collisions.
type contextKey string

const (
	tenantKey contextKey = "tenant_id"
	orgKey    contextKey = "org_id"
	scopeKey  contextKey = "scope"
)

// Scope constants for distinguishing token types.
const (
	ScopeTenant   = "tenant"
	ScopePlatform = "platform"
)

// WithTenantID returns a new context with the tenant ID set.
func WithTenantID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, tenantKey, id)
}

// FromContext returns the tenant ID from context.
// Returns 0 if not set.
func FromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(tenantKey).(int64)
	return id
}

// MustFromContext returns the tenant ID from context or panics if not set.
func MustFromContext(ctx context.Context) int64 {
	id, ok := ctx.Value(tenantKey).(int64)
	if !ok || id == 0 {
		panic("tenant: missing tenant_id in context")
	}
	return id
}

// WithOrgID returns a new context with the organization ID set.
func WithOrgID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, orgKey, id)
}

// OrgFromContext returns the organization ID from context.
// Returns 0 if not set.
func OrgFromContext(ctx context.Context) int64 {
	id, _ := ctx.Value(orgKey).(int64)
	return id
}

// WithScope returns a new context with the scope set.
func WithScope(ctx context.Context, scope string) context.Context {
	return context.WithValue(ctx, scopeKey, scope)
}

// ScopeFromContext returns the scope from context.
// Returns empty string if not set.
func ScopeFromContext(ctx context.Context) string {
	s, _ := ctx.Value(scopeKey).(string)
	return s
}

// IsPlatformScope returns true if the context scope is "platform".
func IsPlatformScope(ctx context.Context) bool {
	return ScopeFromContext(ctx) == ScopePlatform
}
