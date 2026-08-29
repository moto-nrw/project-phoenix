package tenant

import (
	"context"
	"errors"
)

// ErrTenantRequired reports that no validated tenant ID is present.
var ErrTenantRequired = errors.New("tenant: tenant ID is required")

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
	// Zero remains the legacy marker for explicitly unscoped admin work until
	// #2642 removes the primitive helper and its callers.
	if id == 0 {
		return context.WithValue(ctx, tenantKey, id)
	}
	validated, err := NewTenantID(id)
	if err != nil {
		panic(err)
	}
	return WithTenant(ctx, validated)
}

// WithTenant returns a new context containing a validated tenant ID.
func WithTenant(ctx context.Context, id TenantID) context.Context {
	if id.IsZero() {
		panic(ErrTenantRequired)
	}
	return context.WithValue(ctx, tenantKey, id)
}

// FromContext returns the tenant ID from context.
// Returns 0 if not set.
func FromContext(ctx context.Context) int64 {
	id, err := TenantFromContext(ctx)
	if err != nil {
		return 0
	}
	return id.Int64()
}

// TenantFromContext returns the validated tenant ID or ErrTenantRequired.
// The int64 branch keeps legacy contexts readable until #2642 cuts every
// request and worker caller over to WithTenant.
func TenantFromContext(ctx context.Context) (TenantID, error) {
	switch id := ctx.Value(tenantKey).(type) {
	case TenantID:
		if !id.IsZero() {
			return id, nil
		}
	case int64:
		validated, err := NewTenantID(id)
		if err == nil {
			return validated, nil
		}
	}
	return TenantID{}, ErrTenantRequired
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
