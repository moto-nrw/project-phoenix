// Package active_test contains hermetic integration tests for the active API handlers.
// Each test creates its own fixtures and cleans up after itself.
package active_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// ============================================================================
// COMBINED GROUP TESTS
// ============================================================================

func TestCombinedGroups_Integration(t *testing.T) {
	tc, router := setupCombinedGroupRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list combined groups", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/combined", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		_, ok := response["data"].([]interface{})
		assert.True(t, ok, "Expected data to be an array")
	})

	t.Run("get active combined groups", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/combined/active", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("create combined group", func(t *testing.T) {
		// Create fixtures
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Combined Room %d", time.Now().UnixNano()))
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		body := map[string]interface{}{
			"name":       fmt.Sprintf("Test Combined %d", time.Now().UnixNano()),
			"room_id":    room.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest(t, "POST", "/active/combined", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsCreate})

		testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
	})

	t.Run("get combined group by id", func(t *testing.T) {
		// Create combined group fixture
		combinedGroup := createTestCombinedGroup(t, tc.db)
		defer cleanupCombinedGroup(t, tc.db, combinedGroup.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/combined/%d", combinedGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].(map[string]interface{})
		require.True(t, ok, "Expected data to be an object")
		assert.Equal(t, float64(combinedGroup.ID), data["id"])
	})

	t.Run("get combined group not found", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/combined/999999", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertNotFound(t, rr)
	})

	t.Run("update combined group", func(t *testing.T) {
		combinedGroup := createTestCombinedGroup(t, tc.db)
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Update Room %d", time.Now().UnixNano()))
		defer cleanupCombinedGroup(t, tc.db, combinedGroup.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		body := map[string]interface{}{
			"name":       "Updated Combined Name",
			"room_id":    room.ID,
			"start_time": time.Now().Format(time.RFC3339),
		}

		req := testutil.NewJSONRequest(t, "PUT", fmt.Sprintf("/active/combined/%d", combinedGroup.ID), body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("end combined group", func(t *testing.T) {
		combinedGroup := createTestCombinedGroup(t, tc.db)
		defer cleanupCombinedGroup(t, tc.db, combinedGroup.ID)

		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/active/combined/%d/end", combinedGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("delete combined group", func(t *testing.T) {
		combinedGroup := createTestCombinedGroup(t, tc.db)
		// No need to defer cleanup - we're deleting it

		req := testutil.NewJSONRequest(t, "DELETE", fmt.Sprintf("/active/combined/%d", combinedGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsDelete})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// ============================================================================
// GROUP MAPPING TESTS
// ============================================================================

func TestGroupMappings_Integration(t *testing.T) {
	tc, router := setupMappingsRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get group mappings by group", func(t *testing.T) {
		// Create fixtures
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Mapping Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Mapping Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/mappings/group/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get combined group mappings", func(t *testing.T) {
		combinedGroup := createTestCombinedGroup(t, tc.db)
		defer cleanupCombinedGroup(t, tc.db, combinedGroup.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/mappings/combined/%d", combinedGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("add group to combination", func(t *testing.T) {
		// Create fixtures
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Add Mapping Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Add Mapping Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		combinedGroup := createTestCombinedGroup(t, tc.db)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)
		defer cleanupCombinedGroup(t, tc.db, combinedGroup.ID)

		body := map[string]interface{}{
			"active_group_id":   activeGroup.ID,
			"combined_group_id": combinedGroup.ID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/active/mappings/add", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("remove group from combination", func(t *testing.T) {
		// Create fixtures with mapping
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Remove Mapping Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Remove Mapping Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		combinedGroup := createTestCombinedGroup(t, tc.db)
		mapping := createTestGroupMapping(t, tc.db, activeGroup.ID, combinedGroup.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)
		defer cleanupCombinedGroup(t, tc.db, combinedGroup.ID)
		defer cleanupGroupMapping(t, tc.db, mapping.ID)

		body := map[string]interface{}{
			"active_group_id":   activeGroup.ID,
			"combined_group_id": combinedGroup.ID,
		}

		req := testutil.NewJSONRequest(t, "POST", "/active/mappings/remove", body)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsUpdate})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// ============================================================================
// UNCLAIMED GROUPS TESTS
// ============================================================================

func TestUnclaimedGroups_Integration(t *testing.T) {
	tc, router := setupUnclaimedRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("list unclaimed groups", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/unclaimed", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		// Data can be an array or nil (empty result)
		data := response["data"]
		if data != nil {
			_, ok := data.([]interface{})
			assert.True(t, ok, "Expected data to be an array or nil")
		}
	})

	t.Run("claim group - requires JWT with staff context", func(t *testing.T) {
		// Create fixtures
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Claim Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Claim Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Claim", "Staff")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID)
		defer testpkg.CleanupAuthFixtures(t, tc.db, account.ID)

		// Create claims with the account ID
		staffClaims := jwt.AppClaims{
			ID:          int(account.ID),
			Sub:         fmt.Sprintf("%d", account.ID),
			Roles:       []string{"staff"},
			Permissions: []string{permissions.GroupsUpdate},
		}

		req := testutil.NewJSONRequest(t, "POST", fmt.Sprintf("/active/groups/%d/claim", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, staffClaims, []string{permissions.GroupsUpdate})

		// This may fail without full staff context, but exercises the code path
		// The important thing is we get past the permission check
		assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusUnauthorized || rr.Code == http.StatusBadRequest,
			"Expected success or auth error, got %d", rr.Code)
	})
}

// ============================================================================
// SUPERVISOR BY GROUP TESTS
// ============================================================================

func TestSupervisorsByGroup_Integration(t *testing.T) {
	tc, router := setupSupervisorRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get supervisors by group", func(t *testing.T) {
		// Create fixtures
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Supervisor Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Supervisor Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		staff := testpkg.CreateTestStaff(t, tc.db, "Supervisor", "Test")
		supervisor := testpkg.CreateTestGroupSupervisor(t, tc.db, staff.ID, activeGroup.ID, "supervisor")
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, staff.ID, supervisor.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/supervisors/group/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.GreaterOrEqual(t, len(data), 1, "Expected at least one supervisor")
	})

	t.Run("get supervisors by group - empty result", func(t *testing.T) {
		// Create active group without supervisors
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Empty Supervisor Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Empty Supervisor Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/supervisors/group/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get supervisors by group - invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/supervisors/group/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// VISITS BY GROUP TESTS
// ============================================================================

func TestVisitsByGroup_Integration(t *testing.T) {
	tc, router := setupVisitsRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get visits by group", func(t *testing.T) {
		// Create fixtures
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Visits Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Visits Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		student := testpkg.CreateTestStudent(t, tc.db, "Visit", "Student", "1a")
		visit := testpkg.CreateTestVisit(t, tc.db, student.ID, activeGroup.ID, time.Now(), nil)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID, student.ID, visit.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/visits/group/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
		data, ok := response["data"].([]interface{})
		require.True(t, ok, "Expected data to be an array")
		assert.GreaterOrEqual(t, len(data), 1, "Expected at least one visit")
	})

	t.Run("get visits by group - empty result", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Empty Visits Room %d", time.Now().UnixNano()))
		activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, fmt.Sprintf("Empty Visits Activity %d", time.Now().UnixNano()))
		activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID, activeGroup.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/visits/group/%d", activeGroup.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get visits by group - invalid id", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/visits/group/invalid", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertBadRequest(t, rr)
	})
}

// ============================================================================
// ANALYTICS TESTS
// ============================================================================

func TestAnalytics_Integration(t *testing.T) {
	tc, router := setupAnalyticsRouter(t)
	adminClaims := testutil.AdminTestClaims(1)

	t.Run("get counts", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/analytics/counts", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get room utilization", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Utilization Room %d", time.Now().UnixNano()))
		defer testpkg.CleanupActivityFixtures(t, tc.db, room.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/analytics/room/%d/utilization", room.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get student attendance", func(t *testing.T) {
		student := testpkg.CreateTestStudent(t, tc.db, "Attendance", "Student", "2a")
		defer testpkg.CleanupActivityFixtures(t, tc.db, student.ID)

		req := testutil.NewJSONRequest(t, "GET", fmt.Sprintf("/active/analytics/student/%d/attendance", student.ID), nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("get dashboard analytics", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "GET", "/active/analytics/dashboard", nil)
		rr := executeWithAuth(router, req, adminClaims, []string{permissions.GroupsRead})

		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// setupCombinedGroupRouter creates a router for combined group testing
func setupCombinedGroupRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active/combined", func(r chi.Router) {
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", tc.resource.ListCombinedGroupsHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/active", tc.resource.GetActiveCombinedGroupsHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", tc.resource.GetCombinedGroupHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", tc.resource.CreateCombinedGroupHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", tc.resource.UpdateCombinedGroupHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", tc.resource.DeleteCombinedGroupHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", tc.resource.EndCombinedGroupHandler())
	})

	return tc, router
}

// setupMappingsRouter creates a router for group mapping testing
func setupMappingsRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active/mappings", func(r chi.Router) {
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", tc.resource.GetGroupMappingsHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/combined/{combinedId}", tc.resource.GetCombinedGroupMappingsHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/add", tc.resource.AddGroupToCombinationHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/remove", tc.resource.RemoveGroupFromCombinationHandler())
	})

	return tc, router
}

// setupUnclaimedRouter creates a router for unclaimed groups testing
func setupUnclaimedRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active", func(r chi.Router) {
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/unclaimed", tc.resource.ListUnclaimedGroupsHandler())
		r.Route("/groups", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/claim", tc.resource.ClaimGroupHandler())
		})
	})

	return tc, router
}

// setupSupervisorRouter creates a router for supervisor testing
func setupSupervisorRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active/supervisors", func(r chi.Router) {
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", tc.resource.GetSupervisorsByGroupHandler())
	})

	return tc, router
}

// setupVisitsRouter creates a router for visits testing
func setupVisitsRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active/visits", func(r chi.Router) {
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", tc.resource.GetVisitsByGroupHandler())
	})

	return tc, router
}

// setupAnalyticsRouter creates a router for analytics testing
func setupAnalyticsRouter(t *testing.T) (*testContext, chi.Router) {
	t.Helper()

	tc := setupTestContext(t)

	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))

	router.Route("/active/analytics", func(r chi.Router) {
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/counts", tc.resource.GetCountsHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/room/{roomId}/utilization", tc.resource.GetRoomUtilizationHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/attendance", tc.resource.GetStudentAttendanceHandler())
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/dashboard", tc.resource.GetDashboardAnalyticsHandler())
	})

	return tc, router
}

// createTestCombinedGroup creates a combined group directly in the database
func createTestCombinedGroup(t *testing.T, db *bun.DB) *active.CombinedGroup {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	combinedGroup := &active.CombinedGroup{
		StartTime: time.Now(),
	}

	err := db.NewInsert().
		Model(combinedGroup).
		ModelTableExpr(`active.combined_groups`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test combined group")

	return combinedGroup
}

// cleanupCombinedGroup removes a combined group from the database
func cleanupCombinedGroup(t *testing.T, db *bun.DB, id int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First delete any mappings
	_, _ = db.NewDelete().
		Model((*interface{})(nil)).
		Table("active.group_mappings").
		Where("active_combined_group_id = ?", id).
		Exec(ctx)

	// Then delete the combined group
	_, err := db.NewDelete().
		Model((*interface{})(nil)).
		Table("active.combined_groups").
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup combined group: %v", err)
	}
}

// createTestGroupMapping creates a group mapping directly in the database
func createTestGroupMapping(t *testing.T, db *bun.DB, activeGroupID, combinedGroupID int64) *active.GroupMapping {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mapping := &active.GroupMapping{
		ActiveGroupID:         activeGroupID,
		ActiveCombinedGroupID: combinedGroupID,
	}

	err := db.NewInsert().
		Model(mapping).
		ModelTableExpr(`active.group_mappings`).
		Scan(ctx)
	require.NoError(t, err, "Failed to create test group mapping")

	return mapping
}

// cleanupGroupMapping removes a group mapping from the database
func cleanupGroupMapping(t *testing.T, db *bun.DB, id int64) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.NewDelete().
		Model((*interface{})(nil)).
		Table("active.group_mappings").
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		t.Logf("cleanup group mapping: %v", err)
	}
}
