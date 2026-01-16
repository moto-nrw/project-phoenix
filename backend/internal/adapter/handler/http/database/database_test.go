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

	databaseAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/database"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
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
// Note: In production, this endpoint requires JWT auth + system:manage permission.
// These tests mount the handler directly to test basic functionality.
// =============================================================================

func TestGetStats_NoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Use the full router which has JWT middleware
	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil)
	// Remove the default admin token to test unauthenticated access
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

func TestGetStats_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create admin with system:manage permission
	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Admin", "Stats")

	router := chi.NewRouter()
	// Mount handler directly to bypass JWT middleware (we're using test claims)
	router.Get("/stats", ctx.resource.GetStatsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithClaims(testutil.AdminTestClaims(int(admin.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Handler should return stats (status 200)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 for successful stats retrieval")
}
