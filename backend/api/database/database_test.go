// Package database_test tests the database API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package database_test

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	databaseAPI "github.com/moto-nrw/project-phoenix/api/database"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	resource := databaseAPI.NewResource(svc.Database, db, slog.Default())

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// GET STATS TESTS
// Note: In production, this endpoint requires JWT auth + system:manage permission.
// These tests exercise the production Router() with a signed test JWT.
// =============================================================================

func TestGetStats_NoAuth(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

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
	t.Parallel()

	ctx := setupTestContext(t)

	// Create admin with system:manage permission
	admin, _ := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Admin", "Stats")

	router := ctx.resource.Router()

	token := testutil.MintTestJWT(t, testutil.AdminTestClaims(int(admin.ID)))
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithJWTBearer(token),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Handler should return stats (status 200)
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 for successful stats retrieval")
}
