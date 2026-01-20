package tenant

import (
	"context"
)

// CtxKey is the type for context keys to avoid collisions with other packages.
type CtxKey int

const (
	// CtxTenant holds the TenantContext in request context.
	CtxTenant CtxKey = iota

	// CtxPermissions holds the permissions array in request context.
	// Kept separate for fast permission checks without full TenantContext access.
	CtxPermissions
)

// TenantContext holds all tenant-related information for a request.
// This is the primary context object set by the tenant middleware and used
// throughout the request lifecycle for authorization and data filtering.
type TenantContext struct {
	// User identification (from BetterAuth)
	UserID    string
	UserEmail string
	UserName  string

	// Organization (OGS) context
	// OrgID is the BetterAuth organization ID - this is used as ogs_id for RLS.
	OrgID   string
	OrgName string
	OrgSlug string

	// Role and permissions
	// Role is the BetterAuth role name (e.g., "supervisor", "ogsAdmin")
	Role string
	// Permissions are resolved from the role (e.g., ["student:read", "location:read"])
	Permissions []string

	// Hierarchy context (resolved from organization record)
	// Every OGS belongs to a Träger (carrier/provider).
	TraegerID   string
	TraegerName string

	// Some OGS belong to a Büro (administrative office).
	// This is nullable - not all OGS have a Büro.
	BueroID   *string
	BueroName *string
}

// SetTenantContext stores TenantContext in the request context.
// This is called by the tenant middleware after successful authentication.
func SetTenantContext(ctx context.Context, tc *TenantContext) context.Context {
	ctx = context.WithValue(ctx, CtxTenant, tc)
	ctx = context.WithValue(ctx, CtxPermissions, tc.Permissions)
	return ctx
}

// TenantFromCtx retrieves TenantContext from request context.
// Returns nil if no tenant context is set (unauthenticated request).
func TenantFromCtx(ctx context.Context) *TenantContext {
	tc, ok := ctx.Value(CtxTenant).(*TenantContext)
	if !ok {
		return nil
	}
	return tc
}

// PermissionsFromCtx retrieves permissions from request context.
// Returns empty slice if no permissions are set.
func PermissionsFromCtx(ctx context.Context) []string {
	perms, ok := ctx.Value(CtxPermissions).([]string)
	if !ok {
		return []string{}
	}
	return perms
}

// UserIDFromCtx is a convenience helper to get user ID from context.
// Returns empty string if no tenant context is set.
func UserIDFromCtx(ctx context.Context) string {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return ""
	}
	return tc.UserID
}

// OrgIDFromCtx is a convenience helper to get organization (OGS) ID from context.
// This is the value used for RLS filtering (app.ogs_id).
// Returns empty string if no tenant context is set.
func OrgIDFromCtx(ctx context.Context) string {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return ""
	}
	return tc.OrgID
}

// RoleFromCtx is a convenience helper to get the user's role from context.
// Returns empty string if no tenant context is set.
func RoleFromCtx(ctx context.Context) string {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return ""
	}
	return tc.Role
}

// TraegerIDFromCtx is a convenience helper to get the Träger ID from context.
// Returns empty string if no tenant context is set.
func TraegerIDFromCtx(ctx context.Context) string {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return ""
	}
	return tc.TraegerID
}

// BueroIDFromCtx is a convenience helper to get the Büro ID from context.
// Returns nil if no tenant context is set or OGS has no Büro.
func BueroIDFromCtx(ctx context.Context) *string {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return nil
	}
	return tc.BueroID
}

// HasLocationPermission checks if the current user can see location data.
// This is a GDPR-critical check: only supervisor and ogsAdmin roles have
// the location:read permission. Büro and Träger admins do NOT have this
// permission as they manage remotely and have no operational need for
// real-time student location data.
func HasLocationPermission(ctx context.Context) bool {
	return HasPermission(ctx, "location:read")
}

// IsAdmin checks if the current user has any admin role.
// Admins are ogsAdmin, bueroAdmin, or traegerAdmin.
func IsAdmin(ctx context.Context) bool {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return false
	}
	switch tc.Role {
	case "ogsAdmin", "bueroAdmin", "traegerAdmin":
		return true
	default:
		return false
	}
}

// IsSupervisor checks if the current user is a supervisor (front-line staff).
func IsSupervisor(ctx context.Context) bool {
	tc := TenantFromCtx(ctx)
	if tc == nil {
		return false
	}
	return tc.Role == "supervisor"
}

// CanManageStaff checks if the user can manage staff members.
func CanManageStaff(ctx context.Context) bool {
	return HasAnyPermission(ctx, "staff:create", "staff:update", "staff:delete", "staff:invite")
}

// CanManageOGS checks if the user can update OGS settings.
func CanManageOGS(ctx context.Context) bool {
	return HasPermission(ctx, "ogs:update")
}
