package routetest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The claims, token and identity-context helpers. Those that need the token
// package live in the shared fixture catalog, which already links it; this
// package re-exports them so a route test names one vocabulary.
type (
	Claims              = testpkg.Claims
	MFAEnrollmentClaims = testpkg.MFAEnrollmentClaims
)

const (
	MFAEnrollmentScopeTenant   = testpkg.MFAEnrollmentScopeTenant
	MFAEnrollmentScopePlatform = testpkg.MFAEnrollmentScopePlatform
)

var (
	MintTestJWT                 = testpkg.MintTestJWT
	TestTokenAuth               = testpkg.TestTokenAuth
	DefaultTestClaims           = testpkg.DefaultTestClaims
	TeacherTestClaims           = testpkg.TeacherTestClaims
	AdminTestClaims             = testpkg.AdminTestClaims
	AdminTestClaimsForTenant    = testpkg.AdminTestClaimsForTenant
	TenantUserTestClaims        = testpkg.TenantUserTestClaims
	ParentTestClaims            = testpkg.ParentTestClaims
	WithAuthenticatedContext    = testpkg.WithAuthenticatedContext
	AuthenticationContext       = testpkg.AuthenticationContext
	ClaimsFromContext           = testpkg.ClaimsFromContext
	WithEnrollmentClaims        = testpkg.WithEnrollmentClaims
	OpaqueCapabilityFingerprint = testpkg.OpaqueCapabilityFingerprint
	WithRefreshToken            = testpkg.WithRefreshToken
)

// WithPermissions adds permissions to the request context.
func WithPermissions(permissions ...string) RequestOption {
	return func(req *http.Request) {
		ctx := testpkg.WithPermissionsContext(req.Context(), permissions)
		*req = *req.WithContext(ctx)
	}
}

// WithClaims adds JWT claims to the request context.
// Also injects tenant context (mirroring TenantMiddleware) so that
// handler-level WithTenantTx can read the tenant ID. Claims carrying the
// bootstrap tenant follow the test into its own tenant (#2419).
func WithClaims(tb testing.TB, claims Claims) RequestOption {
	claims.TenantID = testpkg.RebaseTenantID(tb, claims.TenantID)
	return func(req *http.Request) {
		ctx := testpkg.WithAuthenticatedContext(req.Context(), claims, nil)
		if claims.TenantID != 0 {
			ctx = tenant.WithTenantID(ctx, claims.TenantID)
		}
		*req = *req.WithContext(ctx)
	}
}

// ExecuteWithAuth signs a JWT for the given claims (used as-is, including any
// permissions they already carry) and executes the request through the router.
func ExecuteWithAuth(t *testing.T, router chi.Router, req *http.Request, claims Claims) *httptest.ResponseRecorder {
	t.Helper()
	req.Header.Set("Authorization", "Bearer "+MintTestJWT(t, claims))
	return ExecuteRequestForTest(t, router, req)
}

// ExecuteWithAuthPermissions folds the given permission set into the claims
// (replacing whatever they carried — an empty slice deliberately produces a
// permissionless token), signs a JWT, and executes the request.
func ExecuteWithAuthPermissions(t *testing.T, router chi.Router, req *http.Request, claims Claims, permissions []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = permissions
	req.Header.Set("Authorization", "Bearer "+MintTestJWT(t, claims))
	return ExecuteRequestForTest(t, router, req)
}
