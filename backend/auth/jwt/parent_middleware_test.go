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

// TestParentMiddleware_AcceptsParentScope verifies the happy path:
// scope=parent + non-zero ID passes through and sets the scope on
// context. tenant_id stays 0 by design — parent tokens aren't bound
// to a single school.
func TestParentMiddleware_AcceptsParentScope(t *testing.T) {
	t.Parallel()

	var gotTenantID int64
	var gotScope string
	var handlerCalled bool

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		gotTenantID = tenant.FromContext(r.Context())
		gotScope = tenant.ScopeFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	claims := jwtpkg.AppClaims{
		ID:    42,
		Sub:   "parent@example.test",
		Scope: tenant.ScopeParent,
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, handlerCalled, "parent-scope token must reach the wrapped handler")
	assert.Equal(t, int64(0), gotTenantID,
		"parent tokens carry tenant_id=0; per-action tenant resolution happens at the handler")
	assert.Equal(t, tenant.ScopeParent, gotScope)
}

// TestParentMiddleware_NoClaims_Unauthorized — request with no claims
// (no Authenticator middleware in front) gets 401.
func TestParentMiddleware_NoClaims_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when no claims present")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestParentMiddleware_RejectsTenantScope — defense-in-depth. A
// tenant-scope token (issued for the staff portal) must NOT be
// accepted on parent routes, even if it somehow reaches the chain.
// The host-only cookie + the proxy already make this leak path
// impossible, but the hard reject closes the gap if either fails.
func TestParentMiddleware_RejectsTenantScope(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("tenant-scope token must NOT reach the wrapped handler")
	}))

	claims := jwtpkg.AppClaims{
		ID:       42,
		Sub:      "staff@example.test",
		TenantID: 100,
		Scope:    tenant.ScopeTenant,
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestParentMiddleware_RejectsOrgScope — org-scope tokens (multi-
// school staff) are also rejected on parent routes.
func TestParentMiddleware_RejectsOrgScope(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("org-scope token must NOT reach the wrapped handler")
	}))

	claims := jwtpkg.AppClaims{
		ID:    42,
		Sub:   "orgadmin@example.test",
		OrgID: 7,
		Scope: tenant.ScopeOrg,
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestParentMiddleware_RejectsPlatformScope — operator (platform)
// tokens are rejected on parent routes. Operator endpoints have
// their own scope check (IsPlatformScope) and don't share the
// parent surface.
func TestParentMiddleware_RejectsPlatformScope(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("platform-scope token must NOT reach the wrapped handler")
	}))

	claims := jwtpkg.AppClaims{
		ID:    42,
		Sub:   "operator@example.test",
		Scope: tenant.ScopePlatform,
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestParentMiddleware_RejectsUnknownScope — a token claiming a scope
// value the system doesn't recognize is rejected the same as known
// non-parent scopes. Future-proofs against a deploy where a new
// scope is added but the parent middleware isn't updated.
func TestParentMiddleware_RejectsUnknownScope(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unknown-scope token must NOT reach the wrapped handler")
	}))

	claims := jwtpkg.AppClaims{
		ID:    42,
		Sub:   "future@example.test",
		Scope: "future-scope-not-yet-defined",
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestParentMiddleware_ZeroID_Unauthorized — claims with ID==0 (a
// malformed or downgraded token) are rejected before the scope check
// runs.
func TestParentMiddleware_ZeroID_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.ParentMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("ID=0 claims must NOT reach the wrapped handler")
	}))

	claims := jwtpkg.AppClaims{
		ID:    0,
		Scope: tenant.ScopeParent, // even with valid scope, ID=0 rejects
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
