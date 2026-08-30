// Package sse_test tests the SSE API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// SSE is a streaming protocol with infinite loops, so tests focus on authentication
// and early error handling that returns before streaming begins.
package sse_test

import (
	"log/slog"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	sseAPI "github.com/moto-nrw/project-phoenix/api/sse"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	hub      *realtime.Hub
	resource *sseAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupSSERoute(t)

	// Create realtime hub
	hub := realtime.NewHub(slog.Default())

	// Create SSE resource with all dependencies
	resource := sseAPI.NewResource(
		hub,
		svc.UserContext,
		db,
		slog.Default(),
	)

	return &testContext{
		db:       db,
		services: svc,
		hub:      hub,
		resource: resource,
	}
}

// =============================================================================
// AUTHENTICATION TESTS
// =============================================================================

func TestSSEEvents_NoAuth(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	// Use the full router which has JWT middleware
	router := ctx.resource.Router()

	// Request without JWT token should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/events", nil)
	req.Header.Del("Authorization")

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing authentication")
}

// =============================================================================
// ROUTER CONFIGURATION TESTS
// =============================================================================

func TestSSERouter_EndpointExists(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

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
	t.Parallel()

	ctx := setupTestContext(t)

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
	t.Parallel()

	ctx := setupTestContext(t)

	// Verify resource was created successfully
	assert.NotNil(t, ctx.resource, "Resource should be created")
	assert.NotNil(t, ctx.hub, "Hub should be created")
}

func TestSSEResource_RouterReturnsValidRouter(t *testing.T) {
	t.Parallel()

	ctx := setupTestContext(t)

	router := ctx.resource.Router()
	assert.NotNil(t, router, "Router should not be nil")
}
