//go:build testhooks

package routetest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var MintTestJWT = testpkg.MintTestJWT

// ExecuteRequestForTest is ExecuteRequest with t's disposable database
// runtime when the test opted into one.
func ExecuteRequestForTest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	ctx := testpkg.WithTestTenantRuntime(t, req.Context())
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	servedBy(ctx, router).ServeHTTP(rr, req.WithContext(ctx))
	return rr
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
