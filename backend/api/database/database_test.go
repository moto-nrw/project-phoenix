// Package database_test tests the database API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks) for integration tests,
// and mock services for error path testing.
package database_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	databaseAPI "github.com/moto-nrw/project-phoenix/api/database"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/services"
	databaseSvc "github.com/moto-nrw/project-phoenix/services/database"
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
// MOCK SERVICE FOR ERROR PATH TESTING
// =============================================================================

// mockDatabaseService implements DatabaseService for testing error paths.
type mockDatabaseService struct {
	stats *databaseSvc.StatsResponse
	err   error
}

func (m *mockDatabaseService) GetStats(_ context.Context) (*databaseSvc.StatsResponse, error) {
	return m.stats, m.err
}

// =============================================================================
// RESOURCE CONSTRUCTOR AND ROUTER TESTS
// =============================================================================

func TestNewResource(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test that NewResource creates a valid resource
	resource := databaseAPI.NewResource(ctx.services.Database)
	require.NotNil(t, resource, "NewResource should return non-nil resource")
	assert.NotNil(t, resource.DatabaseService, "Resource should have DatabaseService set")
}

func TestRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test that Router returns a valid chi.Router
	router := ctx.resource.Router()
	require.NotNil(t, router, "Router should return non-nil chi.Router")

	// Verify the /stats route exists by making a request
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should get 200 (success) not 404 (not found)
	assert.NotEqual(t, http.StatusNotFound, rr.Code, "Router should register /stats endpoint")
}

func TestGetStatsHandler_Accessor(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Test that GetStatsHandler returns a valid handler function
	handler := ctx.resource.GetStatsHandler()
	require.NotNil(t, handler, "GetStatsHandler should return non-nil handler")
}

// =============================================================================
// GET STATS TESTS - AUTHORIZATION
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

// =============================================================================
// GET STATS TESTS - DIFFERENT ADMIN ROLES
// =============================================================================

func TestGetStats_BueroAdmin(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	// Create bueroAdmin tenant context
	bueroAdminCtx := &tenant.TenantContext{
		UserID:      "bueroadmin-user-id",
		UserEmail:   "bueroadmin@example.com",
		UserName:    "Test Büro Admin",
		OrgID:       "test-org-id",
		OrgName:     "Test OGS",
		OrgSlug:     "test-ogs",
		Role:        "bueroAdmin",
		Permissions: []string{"ogs:read", "staff:read"},
		TraegerID:   "test-traeger-id",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(bueroAdminCtx),
	)

	rr := testutil.ExecuteRequest(router, req)

	// bueroAdmin is an admin role, should get 200
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 for bueroAdmin. Body: %s", rr.Body.String())
}

func TestGetStats_TraegerAdmin(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	// Create traegerAdmin tenant context
	traegerAdminCtx := &tenant.TenantContext{
		UserID:      "traegeradmin-user-id",
		UserEmail:   "traegeradmin@example.com",
		UserName:    "Test Träger Admin",
		OrgID:       "test-org-id",
		OrgName:     "Test OGS",
		OrgSlug:     "test-ogs",
		Role:        "traegerAdmin",
		Permissions: []string{"ogs:read", "buero:read", "traeger:read"},
		TraegerID:   "test-traeger-id",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(traegerAdminCtx),
	)

	rr := testutil.ExecuteRequest(router, req)

	// traegerAdmin is an admin role, should get 200
	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 for traegerAdmin. Body: %s", rr.Body.String())
}

func TestGetStats_UnknownRole(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	// Create an unknown/invalid role tenant context
	unknownRoleCtx := &tenant.TenantContext{
		UserID:      "unknown-user-id",
		UserEmail:   "unknown@example.com",
		UserName:    "Unknown User",
		OrgID:       "test-org-id",
		OrgName:     "Test OGS",
		OrgSlug:     "test-ogs",
		Role:        "unknownRole",
		Permissions: []string{},
		TraegerID:   "test-traeger-id",
		TraegerName: "Test Träger",
	}

	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(unknownRoleCtx),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Unknown role is not admin, should get 403
	assert.Equal(t, http.StatusForbidden, rr.Code, "Expected 403 for unknown role")
}

// =============================================================================
// GET STATS TESTS - ERROR HANDLING
// =============================================================================

func TestGetStats_ServiceError(t *testing.T) {
	// Create mock service that returns an error
	mockSvc := &mockDatabaseService{
		stats: nil,
		err:   errors.New("database connection failed"),
	}

	// Create resource with mock service
	resource := databaseAPI.NewResource(mockSvc)

	router := chi.NewRouter()
	router.Get("/stats", resource.GetStatsHandler())

	// Admin user should still get 500 when service fails
	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Handler should return 500 Internal Server Error
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Expected 500 for service error. Body: %s", rr.Body.String())

	// Verify error response format
	var errResp map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &errResp)
	require.NoError(t, err, "Response should be valid JSON")
	assert.Equal(t, "error", errResp["status"], "Error response should have status 'error'")
}

// =============================================================================
// GET STATS TESTS - RESPONSE VALIDATION
// =============================================================================

func TestGetStats_ResponseFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/stats", ctx.resource.GetStatsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/stats", nil,
		testutil.WithTenantContext(testutil.OGSAdminTenantContext("admin@example.com")),
	)

	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusOK, rr.Code, "Expected 200. Body: %s", rr.Body.String())

	// Parse and validate response structure
	var stats databaseSvc.StatsResponse
	err := json.Unmarshal(rr.Body.Bytes(), &stats)
	require.NoError(t, err, "Response should be valid JSON")

	// Verify response has expected fields (values may be 0 or more)
	assert.GreaterOrEqual(t, stats.Students, 0, "Students should be >= 0")
	assert.GreaterOrEqual(t, stats.Teachers, 0, "Teachers should be >= 0")
	assert.GreaterOrEqual(t, stats.Rooms, 0, "Rooms should be >= 0")
	assert.GreaterOrEqual(t, stats.Activities, 0, "Activities should be >= 0")
	assert.GreaterOrEqual(t, stats.Groups, 0, "Groups should be >= 0")
	assert.GreaterOrEqual(t, stats.Roles, 0, "Roles should be >= 0")
	assert.GreaterOrEqual(t, stats.Devices, 0, "Devices should be >= 0")
	assert.GreaterOrEqual(t, stats.PermissionCount, 0, "PermissionCount should be >= 0")
}
