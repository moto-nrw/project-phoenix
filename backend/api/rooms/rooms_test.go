package rooms_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	roomsAPI "github.com/moto-nrw/project-phoenix/api/rooms"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *roomsAPI.Resource
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

	resource := roomsAPI.NewResource(svc.Facilities)

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

// setupRouter creates a Chi router with the given handler and optional RLS middleware.
func setupRouter(handler http.HandlerFunc, urlParam string, db *bun.DB) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	// Add tenant RLS middleware to set app.ogs_id for PostgreSQL
	if db != nil {
		router.Use(testutil.TenantRLSMiddleware(db))
	}
	if urlParam != "" {
		router.Get(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Put(fmt.Sprintf("/{%s}", urlParam), handler)
		router.Delete(fmt.Sprintf("/{%s}", urlParam), handler)
	} else {
		router.Get("/", handler)
		router.Post("/", handler)
	}
	return router
}

// executeWithAuth executes a request with JWT claims, permissions, and tenant context.
// The TenantRLSMiddleware attached to the router will set the database RLS context.
func executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string, tc *tenant.TenantContext) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	if tc != nil {
		ctx = tenant.SetTenantContext(ctx, tc)
	}
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// =============================================================================
// List Rooms Tests
// =============================================================================

func TestListRooms(t *testing.T) {
	tc := setupTestContext(t)
	ogsID := testpkg.SetupTestOGS(t, tc.db)

	// Create test rooms
	room1 := testpkg.CreateTestRoom(t, tc.db, "Test Room 1", ogsID)
	room2 := testpkg.CreateTestRoom(t, tc.db, "Test Room 2", ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room1.ID, room2.ID)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: ogsID, OrgName: "Test OGS"}

	t.Run("success_lists_all_rooms", func(t *testing.T) {
		router := setupRouter(tc.resource.ListRoomsHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_pagination", func(t *testing.T) {
		router := setupRouter(tc.resource.ListRoomsHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/?page=1&page_size=10", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_building_filter", func(t *testing.T) {
		router := setupRouter(tc.resource.ListRoomsHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/?building=Main", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Get Room Tests
// =============================================================================

func TestGetRoom(t *testing.T) {
	tc := setupTestContext(t)

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "Get Room Test", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_gets_room", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHandler(), "id", tc.db)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", room.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Get Room Test", "Response should contain room name")
	})

	t.Run("not_found_for_nonexistent_room", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHandler(), "id", tc.db)
		req := testutil.NewRequest("GET", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHandler(), "id", tc.db)
		req := testutil.NewRequest("GET", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Create Room Tests
// =============================================================================

func TestCreateRoom(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_creates_room", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateRoomHandler(), "", tc.db)
		uniqueName := fmt.Sprintf("Created Room %d", time.Now().UnixNano())
		body := map[string]interface{}{
			"name":     uniqueName,
			"building": "Main",
			"capacity": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), uniqueName, "Response should contain room name")
	})

	t.Run("success_creates_room_with_all_fields", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateRoomHandler(), "", tc.db)
		uniqueName := fmt.Sprintf("Full Room %d", time.Now().UnixNano())
		floor := 2
		body := map[string]interface{}{
			"name":     uniqueName,
			"building": "Main",
			"floor":    floor,
			"capacity": 25,
			"category": "classroom",
			"color":    "#FF5733",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_missing_name", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateRoomHandler(), "", tc.db)
		body := map[string]interface{}{
			"building": "Main",
			"capacity": 30,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Update Room Tests
// =============================================================================

func TestUpdateRoom(t *testing.T) {
	tc := setupTestContext(t)

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "Update Room Test", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_updates_room", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateRoomHandler(), "id", tc.db)
		uniqueName := fmt.Sprintf("Updated Room %d", time.Now().UnixNano())
		body := map[string]interface{}{
			"name":     uniqueName,
			"capacity": 40,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", room.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "Updated Room", "Response should contain updated name")
	})

	t.Run("not_found_for_nonexistent_room", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateRoomHandler(), "id", tc.db)
		body := map[string]interface{}{
			"name": "Test",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/999999", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad_request_missing_name", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateRoomHandler(), "id", tc.db)
		body := map[string]interface{}{
			"capacity": 40,
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", room.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Delete Room Tests
// =============================================================================

func TestDeleteRoom(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_deletes_room", func(t *testing.T) {
		// Create room specifically for deletion
		room := testpkg.CreateTestRoom(t, tc.db, "Delete Room Test", tc.ogsID)

		router := setupRouter(tc.resource.DeleteRoomHandler(), "id", tc.db)
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d", room.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusNoContent, rr.Code, "Expected 204 No Content. Body: %s", rr.Body.String())
	})

	t.Run("error_for_nonexistent_room", func(t *testing.T) {
		router := setupRouter(tc.resource.DeleteRoomHandler(), "id", tc.db)
		req := testutil.NewRequest("DELETE", "/999999", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		// Service returns error when room doesn't exist
		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})

	t.Run("bad_request_for_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.DeleteRoomHandler(), "id", tc.db)
		req := testutil.NewRequest("DELETE", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Get Rooms by Category Tests
// =============================================================================

func TestGetRoomsByCategory(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_gets_rooms_by_category", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomsByCategoryHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/?category=classroom", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_missing_category", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomsByCategoryHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Building and Category List Tests
// =============================================================================

func TestGetBuildingList(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_gets_building_list", func(t *testing.T) {
		router := setupRouter(tc.resource.GetBuildingListHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "buildings", "Response should contain buildings")
	})
}

func TestGetCategoryList(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_gets_category_list", func(t *testing.T) {
		router := setupRouter(tc.resource.GetCategoryListHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
		assert.Contains(t, rr.Body.String(), "categories", "Response should contain categories")
	})
}

// =============================================================================
// Available Rooms Tests
// =============================================================================

func TestGetAvailableRooms(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_gets_available_rooms", func(t *testing.T) {
		router := setupRouter(tc.resource.GetAvailableRoomsHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_capacity_filter", func(t *testing.T) {
		router := setupRouter(tc.resource.GetAvailableRoomsHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/?capacity=20", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}

// =============================================================================
// Room History Tests
// =============================================================================

func TestGetRoomHistory(t *testing.T) {
	tc := setupTestContext(t)

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "History Room Test", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_gets_room_history", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHistoryHandler(), "id", tc.db)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", room.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("success_with_date_range", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHistoryHandler(), "id", tc.db)
		// Use URL-safe format (no colons in time portion) - RFC3339 is supported but needs encoding
		start := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339)
		end := time.Now().UTC().Format(time.RFC3339)
		// Create request with properly encoded query params
		req := httptest.NewRequest("GET", fmt.Sprintf("/%d", room.ID), nil)
		q := req.URL.Query()
		q.Set("start", start)
		q.Set("end", end)
		req.URL.RawQuery = q.Encode()

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})

	t.Run("bad_request_invalid_date_format", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHistoryHandler(), "id", tc.db)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?start=invalid", room.ID), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_start_after_end", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHistoryHandler(), "id", tc.db)
		start := time.Now().Format(time.RFC3339)
		end := time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d?start=%s&end=%s", room.ID, start, end), nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}
