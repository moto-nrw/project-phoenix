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
