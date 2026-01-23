// Package activities_test tests the activities API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package activities_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *activitiesAPI.Resource
	ogsID    string
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)
	ogsID := testpkg.SetupTestOGS(t, db)

	resource := activitiesAPI.NewResource(
		svc.Activities,
		svc.Schedule,
		svc.Users,
		svc.UserContext,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		ogsID:    ogsID,
	}
}

// cleanupActivity cleans up an activity and its related records
func cleanupActivity(t *testing.T, db *bun.DB, activityID int64) {
	t.Helper()
	ctx := context.Background()

	// Delete enrollments (actual table name is student_enrollments)
	_, _ = db.NewDelete().
		TableExpr("activities.student_enrollments").
		Where("activity_group_id = ?", activityID).
		Exec(ctx)

	// Delete schedules
	_, _ = db.NewDelete().
		TableExpr("activities.schedules").
		Where("activity_group_id = ?", activityID).
		Exec(ctx)

	// Delete supervisors (actual table name is supervisors, not supervisors_planned)
	_, _ = db.NewDelete().
		TableExpr("activities.supervisors").
		Where("group_id = ?", activityID).
		Exec(ctx)

	// Delete activity
	_, _ = db.NewDelete().
		TableExpr("activities.groups").
		Where("id = ?", activityID).
		Exec(ctx)
}

// cleanupCategory cleans up a category and any groups referencing it
func cleanupCategory(t *testing.T, db *bun.DB, categoryID int64) {
	t.Helper()
	ctx := context.Background()

	// First delete any groups that reference this category (FK constraint)
	_, _ = db.NewDelete().
		TableExpr("activities.groups").
		Where("category_id = ?", categoryID).
		Exec(ctx)

	// Then delete the category
	_, _ = db.NewDelete().
		TableExpr("activities.categories").
		Where("id = ?", categoryID).
		Exec(ctx)
}

// =============================================================================
// ACTIVITY CRUD TESTS
// =============================================================================

func TestListActivities_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test activity
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestList-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities", ctx.resource.ListActivitiesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	assert.NotEmpty(t, data, "Expected at least one activity")
}

func TestListActivities_WithCategoryFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestFilter-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities", ctx.resource.ListActivitiesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities?category_id=%d", activity.CategoryID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetActivity_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestGet-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities/{id}", ctx.resource.GetActivityHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.Equal(t, float64(activity.ID), data["id"])
}

func TestGetActivity_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/activities/{id}", ctx.resource.GetActivityHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetActivity_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/activities/{id}", ctx.resource.GetActivityHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateActivity_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("CreateTest-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Post("/activities", ctx.resource.CreateActivityHandler())

	body := map[string]interface{}{
		"name":             fmt.Sprintf("NewActivity-%d", time.Now().UnixNano()),
		"max_participants": 15,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup created activity
	if id, ok := data["id"].(float64); ok {
		cleanupActivity(t, ctx.db, int64(id))
	}
}

func TestCreateActivity_BadRequest_MissingName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("BadReq-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Post("/activities", ctx.resource.CreateActivityHandler())

	body := map[string]interface{}{
		"max_participants": 15,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateActivity_BadRequest_MissingCategoryID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/activities", ctx.resource.CreateActivityHandler())

	body := map[string]interface{}{
		"name":             "NoCategoryActivity",
		"max_participants": 15,
		"is_open":          true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestCreateActivity_BadRequest_ZeroParticipants(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("ZeroP-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Post("/activities", ctx.resource.CreateActivityHandler())

	body := map[string]interface{}{
		"name":             "ZeroParticipants",
		"max_participants": 0,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestUpdateActivity_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestUpdate-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Put("/activities/{id}", ctx.resource.UpdateActivityHandler())

	body := map[string]interface{}{
		"name":             fmt.Sprintf("UpdatedActivity-%d", time.Now().UnixNano()),
		"max_participants": 25,
		"is_open":          false,
		"category_id":      activity.CategoryID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/%d", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateActivity_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("NotFound-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Put("/activities/{id}", ctx.resource.UpdateActivityHandler())

	body := map[string]interface{}{
		"name":             "SomeActivity",
		"max_participants": 10,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/activities/99999", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteActivity_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("ToDelete-%d", time.Now().UnixNano()), ctx.ogsID)
	categoryID := activity.CategoryID
	defer cleanupCategory(t, ctx.db, categoryID)

	router := chi.NewRouter()
	router.Delete("/activities/{id}", ctx.resource.DeleteActivityHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/%d", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteActivity_NonExistent_ReturnsSuccess(t *testing.T) {
	// Note: The API uses an idempotent delete pattern - it returns success
	// even if the activity doesn't exist. This is a valid design choice.
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/activities/{id}", ctx.resource.DeleteActivityHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/activities/99999", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// API uses idempotent delete - returns success even for non-existent
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteActivity_InvalidID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Delete("/activities/{id}", ctx.resource.DeleteActivityHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/activities/invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CATEGORY TESTS
// =============================================================================

func TestListCategories_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("TestCat-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Get("/activities/categories", ctx.resource.ListCategoriesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/categories", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	assert.NotEmpty(t, data, "Expected at least one category")
}

func TestGetTimespans_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/activities/timespans", ctx.resource.GetTimespansHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/timespans", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// SCHEDULE TESTS
// =============================================================================

func TestGetActivitySchedules_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("ScheduleTest-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities/{id}/schedules", ctx.resource.GetActivitySchedulesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d/schedules", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestCreateActivitySchedule_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("CreateSched-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Post("/activities/{id}/schedules", ctx.resource.CreateActivityScheduleHandler())

	body := map[string]interface{}{
		"weekday": 1, // Monday
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/schedules", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateActivitySchedule_BadRequest_InvalidWeekday(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("InvalidSched-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Post("/activities/{id}/schedules", ctx.resource.CreateActivityScheduleHandler())

	body := map[string]interface{}{
		"weekday": 10, // Invalid weekday
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/schedules", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetAvailableTimeSlots_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/activities/schedules/available", ctx.resource.GetAvailableTimeSlotsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/schedules/available", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableTimeSlots_BadRequest_InvalidWeekday(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/activities/schedules/available", ctx.resource.GetAvailableTimeSlotsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/schedules/available?weekday=invalid", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SUPERVISOR TESTS
// =============================================================================

func TestGetActivitySupervisors_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("SupervisorTest-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities/{id}/supervisors", ctx.resource.GetActivitySupervisorsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d/supervisors", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestAssignSupervisor_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("AssignSup-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	staff := testpkg.CreateTestStaff(t, ctx.db, "Supervisor", "Test", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Post("/activities/{id}/supervisors", ctx.resource.AssignSupervisorHandler())

	body := map[string]interface{}{
		"staff_id":   staff.ID,
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/supervisors", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestAssignSupervisor_BadRequest_MissingStaffID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("NoStaff-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Post("/activities/{id}/supervisors", ctx.resource.AssignSupervisorHandler())

	body := map[string]interface{}{
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/supervisors", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetAvailableSupervisors_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	staff := testpkg.CreateTestStaff(t, ctx.db, "Available", "Supervisor", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	router := chi.NewRouter()
	router.Get("/activities/supervisors/available", ctx.resource.GetAvailableSupervisorsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/supervisors/available", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// STUDENT ENROLLMENT TESTS
// =============================================================================

func TestGetActivityStudents_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("EnrollTest-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities/{id}/students", ctx.resource.GetActivityStudentsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d/students", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestEnrollStudent_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("EnrollS-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Enroll", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/activities/{id}/students/{studentId}", ctx.resource.EnrollStudentHandler())

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestEnrollStudent_Conflict_AlreadyEnrolled(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("DupEnroll-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Dup", "Enroll", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Post("/activities/{id}/students/{studentId}", ctx.resource.EnrollStudentHandler())

	// First enrollment
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusOK, rr.Code, "First enrollment failed: %s", rr.Body.String())

	// Second enrollment - should conflict
	req = testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)
	rr = testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusConflict, rr.Code, "Expected conflict for duplicate enrollment. Body: %s", rr.Body.String())
}

func TestGetStudentEnrollments_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	student := testpkg.CreateTestStudent(t, ctx.db, "GetEnroll", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Get("/activities/students/{studentId}", ctx.resource.GetStudentEnrollmentsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/students/%d", student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableActivities_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	student := testpkg.CreateTestStudent(t, ctx.db, "Available", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Get("/activities/students/{studentId}/available", ctx.resource.GetAvailableActivitiesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/students/%d/available", student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUnenrollStudent_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("Unenroll-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	student := testpkg.CreateTestStudent(t, ctx.db, "Unenroll", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	// First enroll the student
	enrollRouter := chi.NewRouter()
	enrollRouter.Post("/activities/{id}/students/{studentId}", ctx.resource.EnrollStudentHandler())
	enrollReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)
	enrollRr := testutil.ExecuteRequest(enrollRouter, enrollReq)
	require.Equal(t, http.StatusOK, enrollRr.Code, "Enrollment failed: %s", enrollRr.Body.String())

	// Now unenroll
	router := chi.NewRouter()
	router.Delete("/activities/{id}/students/{studentId}", ctx.resource.UnenrollStudentHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUnenrollStudent_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UnenrollNF-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	student := testpkg.CreateTestStudent(t, ctx.db, "NotEnrolled", "Student", "1a", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student.ID)

	router := chi.NewRouter()
	router.Delete("/activities/{id}/students/{studentId}", ctx.resource.UnenrollStudentHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestBatchEnrollment_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("BatchTest-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	student1 := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Student1", "1a", ctx.ogsID)
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Student2", "1b", ctx.ogsID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, student1.ID, student2.ID)

	router := chi.NewRouter()
	router.Put("/activities/{id}/students", ctx.resource.UpdateGroupEnrollmentsHandler())

	body := map[string]interface{}{
		"student_ids": []int64{student1.ID, student2.ID},
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/%d/students", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestBatchEnrollment_BadRequest_MissingStudentIDs(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("BatchBad-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Put("/activities/{id}/students", ctx.resource.UpdateGroupEnrollmentsHandler())

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/%d/students", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// QUICK CREATE TESTS
// =============================================================================

func TestQuickCreateActivity_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("QuickCreate-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Post("/activities/quick-create", ctx.resource.QuickCreateActivityHandler())

	body := map[string]interface{}{
		"name":             fmt.Sprintf("QuickActivity-%d", time.Now().UnixNano()),
		"category_id":      category.ID,
		"max_participants": 10,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/quick-create", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["activity_id"])

	// Cleanup created activity
	if id, ok := data["activity_id"].(float64); ok {
		cleanupActivity(t, ctx.db, int64(id))
	}
}

func TestQuickCreateActivity_BadRequest_MissingName(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("QuickBad-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupCategory(t, ctx.db, category.ID)

	router := chi.NewRouter()
	router.Post("/activities/quick-create", ctx.resource.QuickCreateActivityHandler())

	body := map[string]interface{}{
		"category_id":      category.ID,
		"max_participants": 10,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/quick-create", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestQuickCreateActivity_BadRequest_MissingCategoryID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/activities/quick-create", ctx.resource.QuickCreateActivityHandler())

	body := map[string]interface{}{
		"name":             "NoCategory",
		"max_participants": 10,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/quick-create", body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ADDITIONAL SCHEDULE TESTS (0% coverage functions)
// =============================================================================

func TestGetActivitySchedule_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("GetSched-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	// Create a schedule first using the service
	actSvc := ctx.services.Activities
	schedData := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         1, // Monday
	}
	schedule, err := actSvc.AddSchedule(context.Background(), activity.ID, schedData)
	require.NoError(t, err)
	require.NotNil(t, schedule)

	router := chi.NewRouter()
	router.Get("/activities/{id}/schedules/{scheduleId}", ctx.resource.GetActivityScheduleHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET",
		fmt.Sprintf("/activities/%d/schedules/%d", activity.ID, schedule.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetActivitySchedule_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("GetSchedNF-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Get("/activities/{id}/schedules/{scheduleId}", ctx.resource.GetActivityScheduleHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET",
		fmt.Sprintf("/activities/%d/schedules/999999", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestUpdateActivitySchedule_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSched-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	// Create a schedule first
	actSvc := ctx.services.Activities
	schedData := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         1, // Monday
	}
	schedule, err := actSvc.AddSchedule(context.Background(), activity.ID, schedData)
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Put("/activities/{id}/schedules/{scheduleId}", ctx.resource.UpdateActivityScheduleHandler())

	body := map[string]interface{}{
		"weekday": 2, // Tuesday
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/schedules/%d", activity.ID, schedule.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateActivitySchedule_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSchedNF-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Put("/activities/{id}/schedules/{scheduleId}", ctx.resource.UpdateActivityScheduleHandler())

	body := map[string]interface{}{
		"weekday": 2,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/schedules/999999", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestDeleteActivitySchedule_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("DelSched-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	// Create a schedule first
	actSvc := ctx.services.Activities
	schedData := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         1, // Monday
	}
	schedule, err := actSvc.AddSchedule(context.Background(), activity.ID, schedData)
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Delete("/activities/{id}/schedules/{scheduleId}", ctx.resource.DeleteActivityScheduleHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/schedules/%d", activity.ID, schedule.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed with 200 or 204
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNoContent,
		"Expected 200 or 204, got %d", rr.Code)
}

func TestDeleteActivitySchedule_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("DelSchedNF-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Delete("/activities/{id}/schedules/{scheduleId}", ctx.resource.DeleteActivityScheduleHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/schedules/999999", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// ADDITIONAL SUPERVISOR TESTS (0% coverage functions)
// =============================================================================

func TestUpdateSupervisorRole_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSupRole-%d", time.Now().UnixNano()), ctx.ogsID)
	staff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("SupRole-%d", time.Now().UnixNano()), "Test", ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Assign supervisor first - get the supervisor record
	actSvc := ctx.services.Activities
	supervisor, err := actSvc.AddSupervisor(context.Background(), activity.ID, staff.ID, false) // false = not primary
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	router := chi.NewRouter()
	router.Put("/activities/{id}/supervisors/{supervisorId}", ctx.resource.UpdateSupervisorRoleHandler())

	body := map[string]interface{}{
		"is_primary": true, // Use is_primary instead of role
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/supervisors/%d", activity.ID, supervisor.ID), body, // Use supervisor.ID
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateSupervisorRole_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSupRoleNF-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Put("/activities/{id}/supervisors/{supervisorId}", ctx.resource.UpdateSupervisorRoleHandler())

	body := map[string]interface{}{
		"role": "primary",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/supervisors/999999", activity.ID), body,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestRemoveSupervisor_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSup-%d", time.Now().UnixNano()), ctx.ogsID)
	staff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("RemSup-%d", time.Now().UnixNano()), "Test", ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)
	defer testpkg.CleanupActivityFixtures(t, ctx.db, staff.ID)

	// Assign supervisor first - get the supervisor record
	actSvc := ctx.services.Activities
	supervisor, err := actSvc.AddSupervisor(context.Background(), activity.ID, staff.ID, false) // false = not primary
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	router := chi.NewRouter()
	router.Delete("/activities/{id}/supervisors/{supervisorId}", ctx.resource.RemoveSupervisorHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/supervisors/%d", activity.ID, supervisor.ID), nil, // Use supervisor.ID
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed with 200 or 204
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNoContent,
		"Expected 200 or 204, got %d: %s", rr.Code, rr.Body.String())
}

func TestRemoveSupervisor_NotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSupNF-%d", time.Now().UnixNano()), ctx.ogsID)
	defer cleanupActivity(t, ctx.db, activity.ID)
	defer cleanupCategory(t, ctx.db, activity.CategoryID)

	router := chi.NewRouter()
	router.Delete("/activities/{id}/supervisors/{supervisorId}", ctx.resource.RemoveSupervisorHandler())

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/supervisors/999999", activity.ID), nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
		testutil.WithTenantContext(testutil.TenantContextWithOrgID(ctx.ogsID)),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}
