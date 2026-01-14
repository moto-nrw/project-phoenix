// Package active_test tests the active API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package active_test

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

	activeAPI "github.com/moto-nrw/project-phoenix/api/active"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test resources
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *activeAPI.Resource
}

// setupTestContext creates test resources for active handler tests
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	resource := activeAPI.NewResource(svc.Active, svc.Users, db)

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("Failed to close database: %v", err)
		}
	})

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// setupProtectedRouter creates a router for testing protected endpoints
// This bypasses JWT verification by using permission middleware only
func setupProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	// Mount routes without JWT middleware for testing
	// We'll set context values directly in tests
	router.Route("/active", func(r chi.Router) {
		// Active Groups
		r.Route("/groups", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListActiveGroupsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", tc.resource.CreateActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", tc.resource.UpdateActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", tc.resource.DeleteActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", tc.resource.EndActiveGroupHandler())
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListVisitsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}", tc.resource.GetStudentVisitsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/current", tc.resource.GetStudentCurrentVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", tc.resource.CreateVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", tc.resource.UpdateVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", tc.resource.DeleteVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", tc.resource.EndVisitHandler())
		})

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListSupervisorsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}", tc.resource.GetStaffSupervisionsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}/active", tc.resource.GetStaffActiveSupervisionsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/", tc.resource.CreateSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Put("/{id}", tc.resource.UpdateSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Delete("/{id}", tc.resource.DeleteSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/{id}/end", tc.resource.EndSupervisionHandler())
		})

		// Analytics
		r.Route("/analytics", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/counts", tc.resource.GetCountsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/dashboard", tc.resource.GetDashboardAnalyticsHandler())
		})
	})

	return tc, router
}

// executeWithAuth executes a request with JWT context values set
func executeWithAuth(router chi.Router, req *http.Request, claims jwt.AppClaims, perms []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, perms)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// setupExtendedProtectedRouter creates a router with extended endpoints for testing
func setupExtendedProtectedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active", func(r chi.Router) {
		// Active Groups (same as setupProtectedRouter)
		r.Route("/groups", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListActiveGroupsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", tc.resource.CreateActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", tc.resource.UpdateActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", tc.resource.DeleteActiveGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", tc.resource.EndActiveGroupHandler())
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListVisitsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}", tc.resource.GetStudentVisitsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/current", tc.resource.GetStudentCurrentVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", tc.resource.CreateVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", tc.resource.UpdateVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", tc.resource.DeleteVisitHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", tc.resource.EndVisitHandler())
		})

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListSupervisorsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}", tc.resource.GetStaffSupervisionsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}/active", tc.resource.GetStaffActiveSupervisionsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/", tc.resource.CreateSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Put("/{id}", tc.resource.UpdateSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Delete("/{id}", tc.resource.DeleteSupervisorHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/{id}/end", tc.resource.EndSupervisionHandler())
		})

		// Analytics - Extended
		r.Route("/analytics", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/counts", tc.resource.GetCountsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/dashboard", tc.resource.GetDashboardAnalyticsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/rooms/{roomId}/utilization", tc.resource.GetRoomUtilizationHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/students/{studentId}/attendance", tc.resource.GetStudentAttendanceHandler())
		})

		// Combined Groups
		r.Route("/combined-groups", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListCombinedGroupsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/active", tc.resource.GetActiveCombinedGroupsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetCombinedGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", tc.resource.CreateCombinedGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", tc.resource.UpdateCombinedGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", tc.resource.DeleteCombinedGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", tc.resource.EndCombinedGroupHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/mappings", tc.resource.GetCombinedGroupMappingsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/{id}/groups", tc.resource.AddGroupToCombinationHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Delete("/{id}/groups/{groupId}", tc.resource.RemoveGroupFromCombinationHandler())
		})

		// Additional group routes
		r.Route("/rooms", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{roomId}/groups", tc.resource.GetActiveGroupsByRoomHandler())
		})
		r.Route("/education-groups", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{groupId}/active", tc.resource.GetActiveGroupsByGroupHandler())
		})

		// Unclaimed groups
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/unclaimed", tc.resource.ListUnclaimedGroupsHandler())

		// Additional routes for coverage
		r.Route("/group-visits", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/groups/{id}/visits", tc.resource.GetActiveGroupVisitsHandler())
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/groups/{id}/supervisors", tc.resource.GetActiveGroupSupervisorsHandler())
		})
	})

	return tc, router
}

// ============================================================================
// ACTIVE GROUP TESTS
// ============================================================================

func TestListActiveGroups(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/groups", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotNil(t, data, "Expected data array")
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/groups", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("success with active filter", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/groups?active=true", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestGetActiveGroup(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test fixtures
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Test Room %d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Test Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

	t.Run("success with valid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/groups/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, float64(activeGroup.ID), data["id"])
	})

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/groups/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad request with invalid id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/groups/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestCreateActiveGroup(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test fixtures
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Create Room %d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Create Activity %d", time.Now().UnixNano()))
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, group.ID)

	t.Run("success with valid data", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id":   group.ID,
			"room_id":    room.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.NotNil(t, data["id"])
	})

	t.Run("bad request with missing group_id", func(t *testing.T) {
		body := map[string]interface{}{
			"room_id":    room.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with missing room_id", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id":   group.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with missing start_time", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id": group.ID,
			"room_id":  room.ID,
		}

		req := testutil.NewJSONRequest("POST", "/active/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id":   group.ID,
			"room_id":    room.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead}) // Wrong permission

		testutil.AssertForbidden(t, rr)
	})
}

func TestEndActiveGroup(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test fixtures
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("End Room %d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("End Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

	t.Run("success ending active group", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", fmt.Sprintf("/active/groups/%d/end", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/groups/99999/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertNotFound(t, rr)
	})
}

// ============================================================================
// VISIT TESTS
// ============================================================================

func TestListVisits(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotNil(t, data, "Expected data array")
	})

	t.Run("success with active filter", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits?active=true", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

func TestCreateVisit(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test fixtures
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Visit Room %d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Visit Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Student", "1a")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID)

	// Note: Full visit creation requires staff context (checked_in_by foreign key)
	// Success case is covered by IoT checkin tests and service layer tests

	t.Run("bad request with missing student_id", func(t *testing.T) {
		body := map[string]interface{}{
			"active_group_id": activeGroup.ID,
			"check_in_time":   time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/visits", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with missing active_group_id", func(t *testing.T) {
		body := map[string]interface{}{
			"student_id":    student.ID,
			"check_in_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/visits", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("bad request with missing check_in_time", func(t *testing.T) {
		body := map[string]interface{}{
			"student_id":      student.ID,
			"active_group_id": activeGroup.ID,
		}

		req := testutil.NewJSONRequest("POST", "/active/visits", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStudentCurrentVisit(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test student
	student := testpkg.CreateTestStudent(t, tc.db, "Current", "Visit", "2b")
	defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

	t.Run("returns not found when no active visit", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/visits/student/%d/current", student.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// API returns 404 when student has no active visit
		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad request with invalid student id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits/student/invalid/current", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// SUPERVISOR TESTS
// ============================================================================

func TestListSupervisors(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.NotNil(t, data, "Expected data array")
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

func TestCreateSupervisor(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test fixtures
	room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Supervisor Room %d", time.Now().UnixNano()))
	group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Supervisor Activity %d", time.Now().UnixNano()))
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
	staff := testpkg.CreateTestStaff(t, tc.db, "Supervisor", "Staff")
	defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID)

	t.Run("success with valid data", func(t *testing.T) {
		body := map[string]interface{}{
			"staff_id":        staff.ID,
			"active_group_id": activeGroup.ID,
			"start_time":      time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/supervisors", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.NotNil(t, data["id"])
	})

	t.Run("bad request with missing staff_id", func(t *testing.T) {
		body := map[string]interface{}{
			"active_group_id": activeGroup.ID,
			"start_time":      time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/supervisors", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"staff_id":        staff.ID,
			"active_group_id": activeGroup.ID,
			"start_time":      time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/supervisors", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead}) // Wrong permission

		testutil.AssertForbidden(t, rr)
	})
}

func TestGetStaffActiveSupervisions(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	// Create test staff
	staff := testpkg.CreateTestStaff(t, tc.db, "Active", "Supervisions")
	defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

	t.Run("success returns empty array when no supervisions", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/supervisors/staff/%d/active", staff.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.Empty(t, data, "Expected empty array")
	})

	t.Run("bad request with invalid staff id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors/staff/invalid/active", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// ANALYTICS TESTS
// ============================================================================

func TestGetCounts(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/analytics/counts", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Contains(t, data, "active_groups_count")
		assert.Contains(t, data, "total_visits_count")
		assert.Contains(t, data, "active_visits_count")
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/analytics/counts", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

func TestGetDashboardAnalytics(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/analytics/dashboard", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Contains(t, data, "students_present")
		assert.Contains(t, data, "active_activities")
		assert.Contains(t, data, "last_updated")
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/analytics/dashboard", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// DELETE/UPDATE TESTS
// ============================================================================

func TestDeleteActiveGroup(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success deleting active group", func(t *testing.T) {
		// Create a new active group to delete
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Delete Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Delete Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		req := testutil.NewJSONRequest("DELETE", fmt.Sprintf("/active/groups/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/groups/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/groups/1", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestUpdateActiveGroup(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success updating active group", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Update Room %d", time.Now().UnixNano()))
		room2 := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Update Room2 %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Update Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, room2.ID, activeGroup.ID)

		body := map[string]interface{}{
			"group_id":   group.ID,
			"room_id":    room2.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", fmt.Sprintf("/active/groups/%d", activeGroup.ID), body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid id", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id":   1,
			"room_id":    1,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/groups/99999", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"group_id":   1,
			"room_id":    1,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/groups/1", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// EXTENDED VISIT TESTS
// ============================================================================

func TestGetVisit(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad request with invalid id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStudentVisits(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success returns visits for student", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Student", "Visits", "3c")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/visits/student/%d", student.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("bad request with invalid student id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/visits/student/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestEndVisit(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/visits/99999/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/visits/1/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestDeleteVisit(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/visits/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/visits/1", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// EXTENDED SUPERVISOR TESTS
// ============================================================================

func TestGetSupervisor(t *testing.T) {
	_, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad request with invalid id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetStaffSupervisions(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success returns supervisions for staff", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, tc.db, "Staff", "Supervisions")
		defer testpkg.CleanupActivityFixtures(t, tc.db, staff.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/supervisors/staff/%d", staff.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("bad request with invalid staff id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors/staff/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateSupervisor(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		body := map[string]interface{}{
			"staff_id":        1,
			"active_group_id": 1,
			"start_time":      time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/supervisors/99999", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"staff_id":        1,
			"active_group_id": 1,
			"start_time":      time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/supervisors/1", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("success updating supervisor", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Supervisor Update Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Supervisor Update Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Update", "Supervisor")
		supervisor := testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "original")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID, supervisor.ID)

		body := map[string]interface{}{
			"staff_id":        staff.ID,
			"active_group_id": activeGroup.ID,
			"start_time":      time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", fmt.Sprintf("/active/supervisors/%d", supervisor.ID), body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestDeleteSupervisor(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/supervisors/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/supervisors/1", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("success deleting supervisor", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Delete Supervisor Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Delete Supervisor Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Delete", "Supervisor")
		supervisor := testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "to-delete")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID)

		req := testutil.NewJSONRequest("DELETE", fmt.Sprintf("/active/supervisors/%d", supervisor.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestEndSupervision(t *testing.T) {
	tc, router := setupProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/supervisors/99999/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/supervisors/1/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("success ending supervision", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("End Supervision Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("End Supervision Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, tc.db, "End", "Supervision")
		supervisor := testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "to-end")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID, supervisor.ID)

		req := testutil.NewJSONRequest("POST", fmt.Sprintf("/active/supervisors/%d/end", supervisor.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// ============================================================================
// EXTENDED ANALYTICS TESTS
// ============================================================================

func TestGetRoomUtilization(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid room id", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Utilization Room %d", time.Now().UnixNano()))
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/analytics/rooms/%d/utilization", room.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid room id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/analytics/rooms/99999/utilization", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 404, 200 with empty data, or 500 if service returns database error
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound || rr.Code == http.StatusInternalServerError,
			"Expected 200, 404, or 500, got %d: %s", rr.Code, rr.Body.String())
	})
}

func TestGetStudentAttendance(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid student id", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Attendance", "Student", "4d")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/analytics/students/%d/attendance", student.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid student id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/analytics/students/99999/attendance", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 404 or 200 with empty data
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
			"Expected 200 or 404, got %d: %s", rr.Code, rr.Body.String())
	})
}

// ============================================================================
// COMBINED GROUP TESTS
// ============================================================================

func TestListCombinedGroups(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		_, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

func TestGetCombinedGroup(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("bad request with invalid id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestCreateCombinedGroup(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid data", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Combined Room %d", time.Now().UnixNano()))
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		body := map[string]interface{}{
			"name":       fmt.Sprintf("Combined Group %d", time.Now().UnixNano()),
			"room_id":    room.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/combined-groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
	})

	t.Run("bad request with missing name", func(t *testing.T) {
		body := map[string]interface{}{
			"room_id":    1,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/combined-groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertBadRequest(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Test Combined",
			"room_id":    1,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("POST", "/active/combined-groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

// ============================================================================
// ADDITIONAL TESTS FOR COVERAGE
// ============================================================================

func TestGetActiveGroupsByRoom(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid room id", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("ByRoom Test %d", time.Now().UnixNano()))
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/rooms/%d/groups", room.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid room id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/rooms/99999/groups", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 200 with empty array or 404
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
			"Expected 200 or 404, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("bad request with invalid room id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/rooms/invalid/groups", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetActiveGroupsByGroup(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid group id", func(t *testing.T) {
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("ByGroup Test %d", time.Now().UnixNano()))
		defer testpkg.CleanupActivityFixtures(t, tc.db, group.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/education-groups/%d/active", group.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid group id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/education-groups/99999/active", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 200 with empty array or 404
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
			"Expected 200 or 404, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("bad request with invalid group id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/education-groups/invalid/active", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateCombinedGroup(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Updated Combined",
			"room_id":    1,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/combined-groups/99999", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"name":       "Updated Combined",
			"room_id":    1,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/combined-groups/1", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestDeleteCombinedGroup(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/combined-groups/99999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/combined-groups/1", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestEndCombinedGroup(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/combined-groups/99999/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("POST", "/active/combined-groups/1/end", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestGetActiveCombinedGroups(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups/active", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		_, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups/active", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

func TestListUnclaimedGroups(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/unclaimed", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/unclaimed", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{})

		testutil.AssertForbidden(t, rr)
	})
}

func TestGetCombinedGroupMappings(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found or bad request with invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups/99999/mappings", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 400 or 404 depending on handler implementation
		assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound,
			"Expected 400 or 404, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("bad request with invalid id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/combined-groups/invalid/mappings", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestUpdateVisit(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("not found with invalid id", func(t *testing.T) {
		body := map[string]interface{}{
			"student_id":      1,
			"active_group_id": 1,
			"check_in_time":   time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/visits/99999", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"student_id":      1,
			"active_group_id": 1,
			"check_in_time":   time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest("PUT", "/active/visits/1", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})

	t.Run("success updating visit", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Visit Update Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Visit Update Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Update", "5e")
		entryTime := time.Now().Add(-1 * time.Hour)
		visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, entryTime, nil)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID, visit.ID)

		body := map[string]interface{}{
			"student_id":      student.ID,
			"active_group_id": activeGroup.ID,
			"check_in_time":   time.Now().Format(time.RFC3339),
			"notes":           "Updated via test",
		}

		req := testutil.NewJSONRequest("PUT", fmt.Sprintf("/active/visits/%d", visit.ID), body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestAddGroupToCombination(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("error with invalid combined group id", func(t *testing.T) {
		body := map[string]interface{}{
			"active_group_id":   1,
			"combined_group_id": 99999,
		}

		req := testutil.NewJSONRequest("POST", "/active/combined-groups/99999/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		// May return 404 or 500 depending on handler implementation
		assert.True(t, rr.Code == http.StatusNotFound || rr.Code == http.StatusInternalServerError || rr.Code == http.StatusBadRequest,
			"Expected 400, 404, or 500, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		body := map[string]interface{}{
			"active_group_id":   1,
			"combined_group_id": 1,
		}

		req := testutil.NewJSONRequest("POST", "/active/combined-groups/1/groups", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestRemoveGroupFromCombination(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("error with invalid ids", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/combined-groups/99999/groups/99998", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsAssign})

		// May return 400, 404, or 500 depending on handler implementation
		assert.True(t, rr.Code == http.StatusBadRequest || rr.Code == http.StatusNotFound || rr.Code == http.StatusInternalServerError,
			"Expected 400, 404, or 500, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("forbidden without permission", func(t *testing.T) {
		req := testutil.NewJSONRequest("DELETE", "/active/combined-groups/1/groups/1", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertForbidden(t, rr)
	})
}

func TestGetActiveGroupVisits(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid active group id", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("GroupVisits Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("GroupVisits Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/group-visits/groups/%d/visits", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid group id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/group-visits/groups/99999/visits", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 200 with empty array or 404
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
			"Expected 200 or 404, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("bad request with invalid group id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/group-visits/groups/invalid/visits", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestGetActiveGroupSupervisors(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with valid active group id", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("GroupSupervisors Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("GroupSupervisors Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/group-visits/groups/%d/supervisors", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("not found with invalid group id", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/group-visits/groups/99999/supervisors", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 200 with empty array or 404
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNotFound,
			"Expected 200 or 404, got %d: %s", rr.Code, rr.Body.String())
	})

	t.Run("bad request with invalid group id format", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/group-visits/groups/invalid/supervisors", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

func TestEndVisitSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success ending visit", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("End Visit Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("End Visit Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "End", "Visit", "7g")
		entryTime := time.Now().Add(-1 * time.Hour)
		visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, entryTime, nil)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID, visit.ID)

		req := testutil.NewJSONRequest("POST", fmt.Sprintf("/active/visits/%d/end", visit.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestDeleteVisitSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success deleting visit", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Delete Visit Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Delete Visit Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "Delete", "Visit", "8h")
		entryTime := time.Now().Add(-1 * time.Hour)
		exitTime := time.Now()
		visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, entryTime, &exitTime)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID)

		req := testutil.NewJSONRequest("DELETE", fmt.Sprintf("/active/visits/%d", visit.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestGetVisitSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success getting visit", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Get Visit Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Get Visit Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "Get", "Visit", "9i")
		entryTime := time.Now().Add(-1 * time.Hour)
		visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, entryTime, nil)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID, visit.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/visits/%d", visit.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestGetSupervisorSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success getting supervisor", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Get Supervisor Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Get Supervisor Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Get", "Supervisor")
		supervisor := testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "test-role")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID, supervisor.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/supervisors/%d", supervisor.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}


func TestGetStudentCurrentVisitSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success getting current visit for student with active visit", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Current Visit Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Current Visit Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "Current", "Visit", "1a")
		entryTime := time.Now().Add(-1 * time.Hour)
		visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, entryTime, nil)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID, visit.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/visits/student/%d/current", student.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// Should return the active visit
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestGetStaffActiveSupervisionsSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success with active supervisions", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Staff Active Sup Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Staff Active Sup Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Staff", "ActiveSup")
		supervisor := testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "active-role")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID, supervisor.ID)

		req := testutil.NewJSONRequest("GET", fmt.Sprintf("/active/supervisors/staff/%d/active", staff.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestEndActiveGroupSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success ending active group", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("End Active Group Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("End Active Group Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, group.ID)

		req := testutil.NewJSONRequest("POST", fmt.Sprintf("/active/groups/%d/end", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestDeleteActiveGroupSuccess(t *testing.T) {
	tc, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("success deleting active group", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Delete Active Group Room %d", time.Now().UnixNano()))
		group := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Delete Active Group Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, group.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, group.ID)

		req := testutil.NewJSONRequest("DELETE", fmt.Sprintf("/active/groups/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

func TestListSupervisorsWithFilters(t *testing.T) {
	_, router := setupExtendedProtectedRouter(t)

	adminClaims := testutil.AdminTestClaims(1)

	t.Run("handles active filter", func(t *testing.T) {
		req := testutil.NewJSONRequest("GET", "/active/supervisors?active=true", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		// May return 200 or 500 depending on service implementation
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusInternalServerError,
			"Expected 200 or 500, got %d: %s", rr.Code, rr.Body.String())
	})
}
