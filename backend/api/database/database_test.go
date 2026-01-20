// Package database_test tests the database API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package database_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	databaseAPI "github.com/moto-nrw/project-phoenix/api/database"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/services"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *databaseAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create database resource
	resource := databaseAPI.NewResource(svc.Database)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// GET STATS TESTS
// Note: In production, this endpoint requires tenant auth + admin role.
// These tests mount the handler directly to test basic functionality.
// =============================================================================

func TestGetStats_NoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	// Request without tenant context should return 403 (not admin)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 for missing tenant context (not admin)")
}

func TestGetStats_NonAdmin(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	// Supervisor (non-admin) should get 403
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(testutil.SupervisorTenantContext("supervisor@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 for non-admin user")
}

func TestGetStats_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	// OGS Admin should get 200
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Handler should return stats (status 200)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 for admin user. Body: %s", rr.Body.String())
}
