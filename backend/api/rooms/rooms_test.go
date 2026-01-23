package rooms_test

import (
	"context"
	"errors"
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
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/services"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// =============================================================================
// Mock FacilityService Implementation for Error Path Testing
// =============================================================================

type mockFacilityService struct {
	// Room operations
	getRoomResult *facilities.Room
	getRoomErr    error

	getRoomWithOccupancyResult facilitiesService.RoomWithOccupancy
	getRoomWithOccupancyErr    error

	createRoomErr error
	updateRoomErr error
	deleteRoomErr error

	listRoomsResult []facilitiesService.RoomWithOccupancy
	listRoomsErr    error

	findRoomByNameResult *facilities.Room
	findRoomByNameErr    error

	findRoomsByBuildingResult []*facilities.Room
	findRoomsByBuildingErr    error

	findRoomsByCategoryResult []*facilities.Room
	findRoomsByCategoryErr    error

	findRoomsByFloorResult []*facilities.Room
	findRoomsByFloorErr    error

	// Advanced operations
	checkRoomAvailabilityResult bool
	checkRoomAvailabilityErr    error

	getAvailableRoomsResult []*facilities.Room
	getAvailableRoomsErr    error

	getAvailableRoomsWithOccupancyResult []facilitiesService.RoomWithOccupancy
	getAvailableRoomsWithOccupancyErr    error

	getRoomUtilizationResult float64
	getRoomUtilizationErr    error

	getBuildingListResult []string
	getBuildingListErr    error

	getCategoryListResult []string
	getCategoryListErr    error

	getRoomHistoryResult []facilitiesService.RoomHistoryEntry
	getRoomHistoryErr    error
}

// Implement TransactionalService interface (ServiceTransactor)
func (m *mockFacilityService) WithTx(_ bun.Tx) interface{} {
	return m
}

func (m *mockFacilityService) GetRoom(_ context.Context, _ int64) (*facilities.Room, error) {
	return m.getRoomResult, m.getRoomErr
}

func (m *mockFacilityService) GetRoomWithOccupancy(_ context.Context, _ int64) (facilitiesService.RoomWithOccupancy, error) {
	return m.getRoomWithOccupancyResult, m.getRoomWithOccupancyErr
}

func (m *mockFacilityService) CreateRoom(_ context.Context, _ *facilities.Room) error {
	return m.createRoomErr
}

func (m *mockFacilityService) UpdateRoom(_ context.Context, _ *facilities.Room) error {
	return m.updateRoomErr
}

func (m *mockFacilityService) DeleteRoom(_ context.Context, _ int64) error {
	return m.deleteRoomErr
}

func (m *mockFacilityService) ListRooms(_ context.Context, _ *base.QueryOptions) ([]facilitiesService.RoomWithOccupancy, error) {
	return m.listRoomsResult, m.listRoomsErr
}

func (m *mockFacilityService) FindRoomByName(_ context.Context, _ string) (*facilities.Room, error) {
	return m.findRoomByNameResult, m.findRoomByNameErr
}

func (m *mockFacilityService) FindRoomsByBuilding(_ context.Context, _ string) ([]*facilities.Room, error) {
	return m.findRoomsByBuildingResult, m.findRoomsByBuildingErr
}

func (m *mockFacilityService) FindRoomsByCategory(_ context.Context, _ string) ([]*facilities.Room, error) {
	return m.findRoomsByCategoryResult, m.findRoomsByCategoryErr
}

func (m *mockFacilityService) FindRoomsByFloor(_ context.Context, _ string, _ int) ([]*facilities.Room, error) {
	return m.findRoomsByFloorResult, m.findRoomsByFloorErr
}

func (m *mockFacilityService) CheckRoomAvailability(_ context.Context, _ int64, _ int) (bool, error) {
	return m.checkRoomAvailabilityResult, m.checkRoomAvailabilityErr
}

func (m *mockFacilityService) GetAvailableRooms(_ context.Context, _ int) ([]*facilities.Room, error) {
	return m.getAvailableRoomsResult, m.getAvailableRoomsErr
}

func (m *mockFacilityService) GetAvailableRoomsWithOccupancy(_ context.Context, _ int) ([]facilitiesService.RoomWithOccupancy, error) {
	return m.getAvailableRoomsWithOccupancyResult, m.getAvailableRoomsWithOccupancyErr
}

func (m *mockFacilityService) GetRoomUtilization(_ context.Context, _ int64) (float64, error) {
	return m.getRoomUtilizationResult, m.getRoomUtilizationErr
}

func (m *mockFacilityService) GetBuildingList(_ context.Context) ([]string, error) {
	return m.getBuildingListResult, m.getBuildingListErr
}

func (m *mockFacilityService) GetCategoryList(_ context.Context) ([]string, error) {
	return m.getCategoryListResult, m.getCategoryListErr
}

func (m *mockFacilityService) GetRoomHistory(_ context.Context, _ int64, _, _ time.Time) ([]facilitiesService.RoomHistoryEntry, error) {
	return m.getRoomHistoryResult, m.getRoomHistoryErr
}

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

	t.Run("bad_request_invalid_id", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHistoryHandler(), "id", tc.db)
		req := testutil.NewRequest("GET", "/invalid", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_end_date_format", func(t *testing.T) {
		router := setupRouter(tc.resource.GetRoomHistoryHandler(), "id", tc.db)
		start := time.Now().AddDate(0, 0, -7).UTC().Format(time.RFC3339)
		// Create request with properly encoded start but invalid end
		req := httptest.NewRequest("GET", fmt.Sprintf("/%d", room.ID), nil)
		q := req.URL.Query()
		q.Set("start", start)
		q.Set("end", "invalid-date")
		req.URL.RawQuery = q.Encode()

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

// =============================================================================
// Mock Service Error Path Tests
// =============================================================================

// setupMockRouter creates a Chi router with a handler using mock service.
func setupMockRouter(handler http.HandlerFunc, urlParam string) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
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

// executeWithAuthMock executes a request with JWT claims and permissions (no tenant context needed for mock).
func executeWithAuthMock(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func TestListRoomsServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		listRoomsErr: errors.New("database connection failed"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.ListRoomsHandler(), "")
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestCreateRoomServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		createRoomErr: errors.New("failed to create room"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.CreateRoomHandler(), "")
		body := map[string]interface{}{
			"name":     "Test Room",
			"building": "Main",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestCreateRoomInvalidBody(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("bad_request_invalid_json", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateRoomHandler(), "", tc.db)
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Content-Type", "application/json")

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_negative_capacity", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateRoomHandler(), "", tc.db)
		body := map[string]interface{}{
			"name":     "Test Room",
			"capacity": -1,
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad_request_invalid_color_format", func(t *testing.T) {
		router := setupRouter(tc.resource.CreateRoomHandler(), "", tc.db)
		body := map[string]interface{}{
			"name":  "Test Room",
			"color": "invalid-color",
		}
		req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateRoomServiceError(t *testing.T) {
	// Test case where GetRoom succeeds but UpdateRoom fails
	room := &facilities.Room{
		Name: "Original Room",
	}
	room.ID = 1
	mockService := &mockFacilityService{
		getRoomResult: room,
		updateRoomErr: errors.New("failed to update room"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_update_failure", func(t *testing.T) {
		router := setupMockRouter(resource.UpdateRoomHandler(), "id")
		body := map[string]interface{}{
			"name": "Updated Room",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/1", body)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestUpdateRoomInvalidID(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("bad_request_invalid_id_format", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateRoomHandler(), "id", tc.db)
		body := map[string]interface{}{
			"name": "Test Room",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", "/invalid", body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateRoomValidationError(t *testing.T) {
	tc := setupTestContext(t)

	// Create test room
	room := testpkg.CreateTestRoom(t, tc.db, "Validation Test Room", tc.ogsID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("bad_request_invalid_color_format", func(t *testing.T) {
		router := setupRouter(tc.resource.UpdateRoomHandler(), "id", tc.db)
		body := map[string]interface{}{
			"name":  "Updated Room",
			"color": "not-a-hex-color",
		}
		req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", room.ID), body)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateRoomInvalidBody(t *testing.T) {
	// Test case where GetRoom succeeds but Bind fails
	room := &facilities.Room{
		Name: "Original Room",
	}
	room.ID = 1
	mockService := &mockFacilityService{
		getRoomResult: room,
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("bad_request_invalid_json", func(t *testing.T) {
		router := setupMockRouter(resource.UpdateRoomHandler(), "id")
		req := httptest.NewRequest("PUT", "/1", nil)
		req.Header.Set("Content-Type", "application/json")

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetRoomsByCategoryServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		findRoomsByCategoryErr: errors.New("database error"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.GetRoomsByCategoryHandler(), "")
		req := testutil.NewRequest("GET", "/?category=classroom", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestGetBuildingListServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		getBuildingListErr: errors.New("database error"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.GetBuildingListHandler(), "")
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestGetCategoryListServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		getCategoryListErr: errors.New("database error"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.GetCategoryListHandler(), "")
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestGetAvailableRoomsServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		getAvailableRoomsErr: errors.New("database error"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.GetAvailableRoomsHandler(), "")
		req := testutil.NewRequest("GET", "/", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})

	t.Run("internal_server_error_with_capacity_filter", func(t *testing.T) {
		router := setupMockRouter(resource.GetAvailableRoomsHandler(), "")
		req := testutil.NewRequest("GET", "/?capacity=20", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})

	t.Run("handles_invalid_capacity_gracefully", func(t *testing.T) {
		// With invalid capacity, it should use capacity=0 and still hit service
		router := setupMockRouter(resource.GetAvailableRoomsHandler(), "")
		req := testutil.NewRequest("GET", "/?capacity=invalid", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})

	t.Run("handles_negative_capacity_gracefully", func(t *testing.T) {
		// With negative capacity, it should use capacity=0 and still hit service
		router := setupMockRouter(resource.GetAvailableRoomsHandler(), "")
		req := testutil.NewRequest("GET", "/?capacity=-5", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestGetRoomHistoryServiceError(t *testing.T) {
	mockService := &mockFacilityService{
		getRoomHistoryErr: errors.New("database error"),
	}
	resource := roomsAPI.NewResource(mockService)

	t.Run("internal_server_error_on_service_failure", func(t *testing.T) {
		router := setupMockRouter(resource.GetRoomHistoryHandler(), "id")
		req := testutil.NewRequest("GET", "/1", nil)

		rr := executeWithAuthMock(router, req, testutil.AdminTestClaims(1), []string{"admin:*"})

		testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	})
}

func TestListRoomsWithCategoryFilter(t *testing.T) {
	tc := setupTestContext(t)

	// Create tenant context
	tenantCtx := &tenant.TenantContext{OrgID: tc.ogsID, OrgName: "Test OGS"}

	t.Run("success_with_category_filter", func(t *testing.T) {
		router := setupRouter(tc.resource.ListRoomsHandler(), "", tc.db)
		req := testutil.NewRequest("GET", "/?category=classroom", nil)

		rr := executeWithAuth(router, req, testutil.AdminTestClaims(1), []string{"admin:*"}, tenantCtx)

		assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	})
}
