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

func TestTenantMiddleware_PopulatesContext(t *testing.T) {
	t.Parallel()

	// Create a handler that inspects the context values
	var gotTenantID int64
	var gotOrgID int64
	var gotScope string

	handler := jwtpkg.TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID = tenant.FromContext(r.Context())
		gotOrgID = tenant.OrgFromContext(r.Context())
		gotScope = tenant.ScopeFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	// Build a request with pre-set claims in context (simulating Authenticator middleware)
	claims := jwtpkg.AppClaims{
		ID:       42,
		Sub:      "test@test.com",
		TenantID: 100,
		OrgID:    10,
		Scope:    "tenant",
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(100), gotTenantID)
	assert.Equal(t, int64(10), gotOrgID)
	assert.Equal(t, "tenant", gotScope)
}

func TestTenantMiddleware_NoClaims_Unauthorized(t *testing.T) {
	t.Parallel()

	handler := jwtpkg.TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called when no claims present")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestTenantMiddleware_ZeroTenantID_StillPasses(t *testing.T) {
	t.Parallel()

	// Old tokens without tenant_id should still be allowed through
	// (the middleware sets 0, which is the default)
	var gotTenantID int64

	handler := jwtpkg.TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTenantID = tenant.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	claims := jwtpkg.AppClaims{
		ID:  42,
		Sub: "test@test.com",
		// No TenantID set (defaults to 0)
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(0), gotTenantID)
}

func TestTenantMiddleware_PlatformScope(t *testing.T) {
	t.Parallel()

	var gotIsPlatform bool

	handler := jwtpkg.TenantMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIsPlatform = tenant.ScopeFromContext(r.Context()) == tenant.ScopePlatform
		w.WriteHeader(http.StatusOK)
	}))

	claims := jwtpkg.AppClaims{
		ID:    42,
		Sub:   "operator@test.com",
		Scope: "platform",
	}
	ctx := context.WithValue(context.Background(), jwtpkg.CtxClaims, claims)
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, gotIsPlatform)
}
