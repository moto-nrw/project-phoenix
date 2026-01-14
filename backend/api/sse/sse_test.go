// Package sse_test tests the SSE API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// SSE is a streaming protocol with infinite loops, so tests focus on authentication
// and early error handling that returns before streaming begins.
package sse_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	sseAPI "github.com/moto-nrw/project-phoenix/api/sse"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	repos    *repositories.Factory
	hub      *realtime.Hub
	resource *sseAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc, err := services.NewFactory(repos, db)
	if err != nil {
		t.Fatalf("Failed to create services factory: %v", err)
	}

	// Create realtime hub
	hub := realtime.NewHub()

	// Create SSE resource with all dependencies
	resource := sseAPI.NewResource(
		hub,
		svc.Active,
		svc.Users,
		svc.UserContext,
	)

	return &testContext{
		db:       db,
		services: svc,
		repos:    repos,
		hub:      hub,
		resource: resource,
	}
}

// =============================================================================
// AUTHENTICATION TESTS
// =============================================================================

func TestSSEEvents_NoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Use the full router which has JWT middleware
	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/events", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

// =============================================================================
// STAFF RESOLUTION TESTS
// Note: Tests that pass auth will enter SSE streaming loop and hang.
// We test staff resolution failure which returns before streaming.
// =============================================================================

func TestSSEEvents_InvalidStaffClaims(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create a person without staff record (just a basic account)
	_, account := testpkg.CreateTestPersonWithAccount(t, ctx.db, "NonStaff", "User")

	// Mount handler directly to bypass JWT middleware
	router := chi.NewRouter()
	router.Get("/events", ctx.resource.EventsHandler())

	// Use teacher claims but with an account ID that doesn't have a staff record
	req := testutil.NewAuthenticatedRequest(t, "GET", "/events", nil,
		testutil.WithClaims(testutil.TeacherTestClaims(int(account.ID))),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Staff resolution should fail for non-staff users
	// Handler returns 403 Forbidden when user is not a staff member
	assert.Equal(t, http.StatusForbidden, rr.Code,
		"Expected 403 for non-staff user")
}

// =============================================================================
// ROUTER CONFIGURATION TESTS
// =============================================================================

func TestSSERouter_EndpointExists(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	// Verify the /events endpoint is registered
	// Without auth, should get 401 (endpoint exists but requires auth)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/events", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	// 401 means endpoint exists but requires authentication
	// 404 would mean endpoint doesn't exist
	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"Expected 401 indicating endpoint exists but requires auth")
}

func TestSSERouter_WrongMethod(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()

	// POST to /events should return 405 Method Not Allowed
	req := testutil.NewAuthenticatedRequest(t, "POST", "/events", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	// Could be 401 (auth check first) or 405 (method check first)
	// Either is acceptable - the key is it's not 200
	assert.Contains(t, []int{http.StatusUnauthorized, http.StatusMethodNotAllowed}, rr.Code,
		"Expected 401 or 405 for POST to SSE endpoint")
}

// =============================================================================
// RESOURCE TESTS
// =============================================================================

func TestSSEResource_Creation(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Verify resource was created successfully
	assert.NotNil(t, ctx.resource, "Resource should be created")
	assert.NotNil(t, ctx.hub, "Hub should be created")
}

func TestSSEResource_RouterReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}
