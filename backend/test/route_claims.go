package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
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
type Claims = jwt.AppClaims

// WithAuthenticatedContext puts the claims and permissions on a context the
// way the auth middleware does. It is the context-level twin of routetest.WithClaims,
// for a handler exercised directly instead of through a request.
func WithAuthenticatedContext(ctx context.Context, claims Claims, permissions []string) context.Context {
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	if permissions != nil {
		ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	}
	return ctx
}

// WithPermissionsContext puts a permission set on ctx the way the auth
// middleware does, without claims.
func WithPermissionsContext(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, jwt.CtxPermissions, permissions)
}

// MintTestJWT signs a JWT for the given claims using the same configuration as
// production (jwt.NewTokenAuth reads the JWT secret from viper / env). Pair it
// with routetest.WithJWTBearer when calling handlers through Resource.Router() so the
// production auth middleware accepts the request.
//
// Callers must arrange for a non-empty auth_jwt_secret to be present before the
// Resource is constructed (typically via routetest.SeedTestJWTConfig in init() or
// TestMain). Without a secret, jwx refuses to HMAC-sign and this helper fails.
//
// Claims carrying the bootstrap tenant are rebased onto the tenant the test
// owns, so a test that opted into a per-test tenant gets a matching token
// without passing the tenant through every claims helper (#2419).
func MintTestJWT(t testing.TB, claims jwt.AppClaims) string {
	t.Helper()
	claims.TenantID = RebaseTenantID(t, claims.TenantID)
	tokenAuth, err := ConfiguredTokenAuth()
	require.NoError(t, err, "MintTestJWT: NewTokenAuth")
	token, err := tokenAuth.CreateJWT(claims)
	require.NoError(t, err, "MintTestJWT: CreateJWT")
	return token
}

// AuthenticationContext returns the identity values injected by request
// options, preferring an explicit test permission set over claims defaults.
func AuthenticationContext(ctx context.Context) (jwt.AppClaims, []string) {
	claims := jwt.ClaimsFromCtx(ctx)
	if granted := jwt.PermissionsFromCtx(ctx); granted != nil {
		return claims, granted
	}
	return claims, claims.Permissions
}

// DefaultTestClaims returns default JWT claims for testing.
func DefaultTestClaims() jwt.AppClaims {
	return jwt.AppClaims{
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
func TeacherTestClaims(accountID int) jwt.AppClaims {
	return jwt.AppClaims{
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
func AdminTestClaims(accountID int) jwt.AppClaims {
	return AdminTestClaimsForTenant(accountID, 1)
}

// AdminTestClaimsForTenant returns admin JWT claims scoped to a specific tenant.
// Use this for tests that run in an isolated tenant (e.g., to avoid cross-package
// fixture interference) instead of the default test tenant id=1.
func AdminTestClaimsForTenant(accountID int, tenantID int64) jwt.AppClaims {
	return jwt.AppClaims{
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
func TenantUserTestClaims(accountID int, tenantID int64, permissions ...string) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          accountID,
		Sub:         "user@example.com",
		Roles:       []string{"user"},
		TenantID:    tenantID,
		Permissions: permissions,
	}
}

// ParentTestClaims returns the claims of a guardian account in the
// cross-tenant parent scope, as the parent portal mints them.
func ParentTestClaims(accountID int) jwt.AppClaims {
	return jwt.AppClaims{
		ID:    accountID,
		Sub:   "parent@example.com",
		Roles: []string{"guardian"},
		Scope: tenant.ScopeParent,
	}
}

// TestTokenAuth returns the signer of the seeded test configuration.
func TestTokenAuth(tb testing.TB) *jwt.TokenAuth {
	tb.Helper()
	tokenAuth, err := ConfiguredTokenAuth()
	require.NoError(tb, err)
	return tokenAuth
}

// ClaimsFromContext returns the claims the auth middleware put on ctx.
func ClaimsFromContext(ctx context.Context) Claims {
	return jwt.ClaimsFromCtx(ctx)
}

// MFAEnrollmentClaims is the enrollment-only token's claims.
type MFAEnrollmentClaims = jwt.MFAEnrollmentClaims

// Enrollment token scopes.
const (
	MFAEnrollmentScopeTenant   = jwt.MFAEnrollmentScopeTenant
	MFAEnrollmentScopePlatform = jwt.MFAEnrollmentScopePlatform
)

// WithEnrollmentClaims puts enrollment-only claims on ctx the way the
// enrollment authenticator does.
func WithEnrollmentClaims(ctx context.Context, claims MFAEnrollmentClaims) context.Context {
	return context.WithValue(ctx, jwt.CtxEnrollmentClaims, claims)
}

// OpaqueCapabilityFingerprint returns the stored fingerprint of an opaque
// capability token.
func OpaqueCapabilityFingerprint(token string) string {
	return jwt.OpaqueCapabilityFingerprint(token)
}

// WithRefreshToken puts the raw refresh token on ctx the way the refresh
// authenticator does, so a refresh handler exercised directly reads it
// without the test naming the token package (#2736).
func WithRefreshToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, jwt.CtxRefreshToken, token)
}
