package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The claims, token and identity-context helpers of route tests. They live
// beside ConfiguredTokenAuth and RebaseTenantID because they need the token
// package, which this fixture catalog already links; api/testutil/routetest
// re-exports them so a route test that composes no services does not link
// api/testutil.

// Claims is the authenticated caller as the router sees it. Aliased here so a
// route test names the claims through the test boundary that mints them,
// rather than reaching for the token package itself.
type Claims = sessiontoken.AppClaims

// WithAuthenticatedContext puts the claims and permissions on a context the
// way the auth middleware does. It is the context-level twin of routetest.WithClaims,
// for a handler exercised directly instead of through a request.
func WithAuthenticatedContext(ctx context.Context, claims Claims, permissions []string) context.Context {
	ctx = context.WithValue(ctx, sessiontoken.CtxClaims, claims)
	if permissions != nil {
		ctx = context.WithValue(ctx, sessiontoken.CtxPermissions, permissions)
	}
	return ctx
}

// WithPermissionsContext puts a permission set on ctx the way the auth
// middleware does, without claims.
func WithPermissionsContext(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, sessiontoken.CtxPermissions, permissions)
}

// AuthenticationContext returns the identity values injected by request
// options, preferring an explicit test permission set over claims defaults.
func AuthenticationContext(ctx context.Context) (sessiontoken.AppClaims, []string) {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	if granted := sessiontoken.PermissionsFromCtx(ctx); granted != nil {
		return claims, granted
	}
	return claims, claims.Permissions
}

// DefaultTestClaims returns default JWT claims for testing.
func DefaultTestClaims() sessiontoken.AppClaims {
	return sessiontoken.AppClaims{
		ID:          1,
		Sub:         "test@example.com",
		Username:    "testuser",
		FirstName:   "Test",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
		TenantID:    1,
	}
}

// TeacherTestClaims returns JWT claims for a teacher user.
func TeacherTestClaims(accountID int) sessiontoken.AppClaims {
	return sessiontoken.AppClaims{
		ID:          accountID,
		Sub:         "teacher@example.com",
		Username:    "teacher",
		FirstName:   "Test",
		LastName:    "Teacher",
		Roles:       []string{"user"},
		Permissions: []string{"students:read", "groups:read", "groups:update", "groups:list", "visits:read", "visits:create", "visits:update", "visits:delete", "visits:list", "activities:update", "activities:delete", "activities:list", "activities:manage", "activities:enroll", "activities:assign", "users:list", "rooms:list", "schedules:read", "schedules:list", "feedback:read", "feedback:list", "substitutions:read"},
		TenantID:    1,
	}
}

// AdminTestClaims returns JWT claims for an admin user.
func AdminTestClaims(accountID int) sessiontoken.AppClaims {
	return AdminTestClaimsForTenant(accountID, 1)
}

// AdminTestClaimsForTenant returns admin JWT claims scoped to a specific tenant.
// Use this for tests that run in an isolated tenant (e.g., to avoid cross-package
// fixture interference) instead of the default test tenant id=1.
func AdminTestClaimsForTenant(accountID int, tenantID int64) sessiontoken.AppClaims {
	return sessiontoken.AppClaims{
		ID:          accountID,
		Sub:         "admin@example.com",
		Username:    "admin",
		FirstName:   "Admin",
		LastName:    "User",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
		IsAdmin:     true,
		TenantID:    tenantID,
	}
}

// TenantUserTestClaims returns the claims of a regular (non-admin) staff
// account on the given tenant that holds exactly the listed permissions.
func TenantUserTestClaims(accountID int, tenantID int64, permissions ...string) sessiontoken.AppClaims {
	return sessiontoken.AppClaims{
		ID:          accountID,
		Sub:         "user@example.com",
		Roles:       []string{"user"},
		TenantID:    tenantID,
		Permissions: permissions,
	}
}

// ParentTestClaims returns the claims of a guardian account in the
// cross-tenant parent scope, as the parent portal mints them.
func ParentTestClaims(accountID int) sessiontoken.AppClaims {
	return sessiontoken.AppClaims{
		ID:    accountID,
		Sub:   "parent@example.com",
		Roles: []string{"guardian"},
		Scope: tenant.ScopeParent,
	}
}

// TestTokenAuth returns the signer of the seeded test configuration.
func TestTokenAuth(tb testing.TB) *sessiontoken.TokenAuth {
	tb.Helper()
	tokenAuth, err := ConfiguredTokenAuth()
	require.NoError(tb, err)
	return tokenAuth
}

// PermissionsFromContext returns the permission set a request option or the
// auth middleware put on ctx, without falling back to the claims; nil when
// none was put there.
func PermissionsFromContext(ctx context.Context) []string {
	permissions, _ := ctx.Value(sessiontoken.CtxPermissions).([]string)
	return permissions
}

// ClaimsFromContext returns the claims the auth middleware put on ctx.
func ClaimsFromContext(ctx context.Context) Claims {
	return sessiontoken.ClaimsFromCtx(ctx)
}

// MFAEnrollmentClaims is the enrollment-only token's claims.
type MFAEnrollmentClaims = sessiontoken.MFAEnrollmentClaims

// Enrollment token scopes.
const (
	MFAEnrollmentScopeTenant   = sessiontoken.MFAEnrollmentScopeTenant
	MFAEnrollmentScopePlatform = sessiontoken.MFAEnrollmentScopePlatform
)

// WithEnrollmentClaims puts enrollment-only claims on ctx the way the
// enrollment authenticator does.
func WithEnrollmentClaims(ctx context.Context, claims MFAEnrollmentClaims) context.Context {
	return context.WithValue(ctx, sessiontoken.CtxEnrollmentClaims, claims)
}

// OpaqueCapabilityFingerprint returns the stored fingerprint of an opaque
// capability token.
func OpaqueCapabilityFingerprint(token string) string {
	return sessiontoken.OpaqueCapabilityFingerprint(token)
}

// WithRefreshToken puts the raw refresh token on ctx the way the refresh
// authenticator does, so a refresh handler exercised directly reads it
// without the test naming the token package (#2736).
func WithRefreshToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, sessiontoken.CtxRefreshToken, token)
}

// WithRequestIdentityCache attaches the native request identity memo for
// behavior tests; entry contents and cache lifetime remain with their owners.
func WithRequestIdentityCache(ctx context.Context) context.Context {
	return securityruntime.WithRequestIdentityCache(ctx)
}
