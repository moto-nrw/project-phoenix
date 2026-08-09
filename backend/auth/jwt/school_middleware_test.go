package jwt_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtpkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

// schoolMiddlewareRequest builds a request whose context carries the
// given claims, mirroring what the Authenticator middleware would set.
func schoolMiddlewareRequest(claims jwtpkg.AppClaims) *http.Request {
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	return httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
}

// TestSchoolMiddleware_AcceptsSchoolScope verifies the happy path:
// scope=school + non-zero ID + non-zero tenant_id passes through and
// sets tenant id, org id, and scope on the context — school tokens are
// tenant-bound (unlike parent tokens), because the class-day surface
// runs under RLS and needs the tenant transaction.
func TestSchoolMiddleware_AcceptsSchoolScope(t *testing.T) {
	var gotTenantID, gotOrgID int64
	var gotScope string
	var handlerCalled bool

	handler := jwtpkg.SchoolMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		gotTenantID = tenant.FromContext(r.Context())
		gotOrgID = tenant.OrgFromContext(r.Context())
		gotScope = tenant.ScopeFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, schoolMiddlewareRequest(jwtpkg.AppClaims{
		ID:       42,
		Sub:      "lehrkraft@example.test",
		Scope:    tenant.ScopeSchool,
		TenantID: 100,
		OrgID:    7,
	}))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, handlerCalled, "school-scope token must reach the wrapped handler")
	assert.Equal(t, int64(100), gotTenantID,
		"school tokens are tenant-bound; the middleware must set the tenant id for TenantTxMiddleware")
	assert.Equal(t, int64(7), gotOrgID)
	assert.Equal(t, tenant.ScopeSchool, gotScope)
}

// TestSchoolMiddleware_NoClaims_Unauthorized — request with no claims
// (no Authenticator middleware in front) gets 401.
func TestSchoolMiddleware_NoClaims_Unauthorized(t *testing.T) {
	handler := jwtpkg.SchoolMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when no claims present")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestSchoolMiddleware_RejectsOtherScopes — defense-in-depth. Tenant,
// org, platform, parent, and unknown scopes must NOT be accepted on
// school routes, even if a token somehow reaches the chain. The
// host-only school cookie + the proxy already make this leak path
// impossible, but the hard reject closes the gap if either fails.
func TestSchoolMiddleware_RejectsOtherScopes(t *testing.T) {
	cases := []struct {
		name   string
		claims jwtpkg.AppClaims
	}{
		{"tenant scope", jwtpkg.AppClaims{ID: 42, Sub: "staff@example.test", TenantID: 100, Scope: tenant.ScopeTenant}},
		{"org scope", jwtpkg.AppClaims{ID: 42, Sub: "orgadmin@example.test", OrgID: 7, Scope: tenant.ScopeOrg}},
		{"platform scope", jwtpkg.AppClaims{ID: 42, Sub: "operator@example.test", Scope: tenant.ScopePlatform}},
		{"parent scope", jwtpkg.AppClaims{ID: 42, Sub: "parent@example.test", Scope: tenant.ScopeParent}},
		{"unknown scope", jwtpkg.AppClaims{ID: 42, Sub: "future@example.test", TenantID: 100, Scope: "future-scope-not-yet-defined"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := jwtpkg.SchoolMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatalf("%s token must NOT reach the wrapped handler", tc.name)
			}))

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, schoolMiddlewareRequest(tc.claims))

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// TestSchoolMiddleware_ZeroTenantID_Unauthorized — a school-scope token
// without a tenant binding is malformed by construction (the school
// login always pins the resolved school). Rejecting it here beats
// letting RLS silently return zero rows downstream.
func TestSchoolMiddleware_ZeroTenantID_Unauthorized(t *testing.T) {
	handler := jwtpkg.SchoolMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("tenant_id=0 school token must NOT reach the wrapped handler")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, schoolMiddlewareRequest(jwtpkg.AppClaims{
		ID:    42,
		Sub:   "lehrkraft@example.test",
		Scope: tenant.ScopeSchool,
	}))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestSchoolMiddleware_ZeroID_Unauthorized — claims with ID==0 (a
// malformed or downgraded token) are rejected before the scope check
// runs.
func TestSchoolMiddleware_ZeroID_Unauthorized(t *testing.T) {
	handler := jwtpkg.SchoolMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("ID=0 claims must NOT reach the wrapped handler")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, schoolMiddlewareRequest(jwtpkg.AppClaims{
		ID:       0,
		Scope:    tenant.ScopeSchool, // even with valid scope, ID=0 rejects
		TenantID: 100,
	}))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestTenantMiddleware_RejectsSchoolScope — the symmetric guard:
// school-scope tokens must NOT pass the tenant middleware. School
// tokens carry a real tenant_id, so without this check they would
// satisfy RLS on every tenant route and quietly widen class_day:read
// into full portal access.
func TestTenantMiddleware_RejectsSchoolScope(t *testing.T) {
	handler := jwtpkg.TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("school-scope token must NOT reach a tenant handler")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, schoolMiddlewareRequest(jwtpkg.AppClaims{
		ID:       42,
		Sub:      "lehrkraft@example.test",
		Scope:    tenant.ScopeSchool,
		TenantID: 100,
	}))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
