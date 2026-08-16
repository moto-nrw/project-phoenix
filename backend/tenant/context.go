package tenant

import "context"

// contextKey is a private type for tenant context keys to prevent collisions.
type contextKey string

const (
	tenantKey  contextKey = "tenant_id"
	orgKey     contextKey = "org_id"
	scopeKey   contextKey = "scope"
	adminTxKey contextKey = "admin_tx"
)

// Scope constants for distinguishing token types.
// Empty string ("") is the default scope for regular tenant users (existing JWTs have no scope field).
// ScopeTenant is intentionally "" to match this convention.
const (
	ScopeTenant   = ""         // Regular user within a single school
	ScopeOrg      = "org"      // Organization-level user (sees all schools in org)
	ScopePlatform = "platform" // Platform operator (moto DevOps, sees everything)
	ScopeParent   = "parent"   // Cross-tenant guardian — sees children across all linked schools
	ScopeSchool   = "school"   // School portal (Lehrkraft) — tenant-bound like ScopeTenant, but only valid on /school/* (#2207)
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

// withAdminTxFlag marks ctx as running inside WithAdminTx (BYPASSRLS).
func withAdminTxFlag(ctx context.Context) context.Context {
	return context.WithValue(ctx, adminTxKey, true)
}

// IsAdminTx reports whether ctx is inside a WithAdminTx callback.
// A leftover tenant_id on the same context does not change this.
func IsAdminTx(ctx context.Context) bool {
	ok, _ := ctx.Value(adminTxKey).(bool)
	return ok
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
