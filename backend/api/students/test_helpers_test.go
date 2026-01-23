package students_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *studentsAPI.Resource
	ogsID    string
}

// setupTestContext initializes the test environment.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	ogsID := testpkg.SetupTestOGS(t, db)

	repoFactory := repositories.NewFactory(db)
	svc, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")

	resource := studentsAPI.NewResource(
		svc.Users, // PersonService
		repoFactory.Student,
		svc.Education,
		svc.UserContext,
		svc.Active,
		svc.IoT,
		repoFactory.PrivacyConsent,
	)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// setupRouter creates a Chi router with the given handler.
// This version uses the testContext to add RLS middleware.
func (tc *testContext) setupRouter(handler http.HandlerFunc, urlParam string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// Add RLS middleware to set database session variable for multitenancy
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantCtx := tenant.TenantFromCtx(r.Context())
			if tenantCtx != nil && tenantCtx.OrgID != "" {
				// Set RLS context for PostgreSQL
				query := fmt.Sprintf("SET LOCAL app.ogs_id = '%s'", tenantCtx.OrgID)
				_, _ = tc.db.ExecContext(r.Context(), query)
			}
			next.ServeHTTP(w, r)
		})
	})

	if urlParam != "" {
		router.Get(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Put(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Delete(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Post(fmt.Sprintf("/{%s}", urlParam), handler)
	} else {
		router.Get("/", handler)
		router.Post("/", handler)
	}
	return router
}

// setupRouter creates a Chi router with the given handler (standalone version for backwards compatibility).
func setupRouter(handler http.HandlerFunc, urlParam string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	if urlParam != "" {
		router.Get(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Put(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Delete(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Post(fmt.Sprintf("/{%s}", urlParam), handler)
	} else {
		router.Get("/", handler)
		router.Post("/", handler)
	}
	return router
}

// executeWithAuth executes a request with JWT claims, permissions, and tenant context.
// The student handlers use tenant context for authorization.
// This standalone version is kept for backwards compatibility.
func executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	return executeWithAuthAndOGS(router, req, claims, permissions, "test-org")
}

// executeWithAuthAndOGS executes a request with JWT claims, permissions, tenant context, and specific OGS ID.
func executeWithAuthAndOGS(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string, ogsID string) *httptest.ResponseRecorder {
	// Create tenant context with permissions
	tc := &tenant.TenantContext{
		UserID:      fmt.Sprintf("user-%d", claims.ID),
		UserEmail:   claims.Username + "@example.com",
		UserName:    claims.Username,
		OrgID:       ogsID,
		OrgName:     "Test Organization",
		OrgSlug:     ogsID,
		Role:        "supervisor",
		Permissions: permissions,
		TraegerID:   "test-traeger",
		TraegerName: "Test Träger",
	}

	// Set both JWT claims (for userContextService) and tenant context (for permission middleware)
	ctx := tenant.SetTenantContext(req.Context(), tc)
	ctx = setJWTClaims(ctx, claims)
	ctx = setJWTPermissions(ctx, permissions)

	// Set OGS ID for transaction handler to pick up
	ctx = base.ContextWithOGSID(ctx, ogsID)

	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// executeWithAuth executes a request with tenant context from testContext.
// This is a method on testContext that automatically uses the correct OGS ID.
func (tc *testContext) executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	return executeWithAuthAndOGS(router, req, claims, permissions, tc.ogsID)
}

// setJWTClaims adds JWT claims to context (for userContextService compatibility).
func setJWTClaims(ctx context.Context, claims jwt.AppClaims) context.Context {
	return context.WithValue(ctx, jwt.CtxClaims, claims)
}

// setJWTPermissions adds JWT permissions to context (for userContextService compatibility).
func setJWTPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, jwt.CtxPermissions, permissions)
}
