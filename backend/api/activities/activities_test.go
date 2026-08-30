// Package activities_test tests the activities API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks). Requests run through the
// production middleware chain by mounting Resource.Router() (Verifier → Authenticator →
// TenantMiddleware → TenantTxMiddleware) — the same code path the real server uses.
package activities_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	activitiesAPI "github.com/moto-nrw/project-phoenix/api/activities"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// init seeds JWT viper defaults before any test constructs a Resource via
// jwt.MustNewTokenAuth() (inside Router()). CI runs without a .env so
// AUTH_JWT_SECRET is unset; without a secret jwx refuses HMAC signing.
func init() {
	testutil.SeedTestJWTConfig()
}

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *activitiesAPI.Resource
	router   chi.Router
}

// setupTestContext initializes test database, services, resource, and a router
// that serves the resource through the production middleware chain. The router
// is mounted at /activities so request URLs match the real API paths.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupActivitiesRoute(t)

	resource := activitiesAPI.NewResource(
		svc.Activities,
		svc.Schedule,
		svc.Users,
		svc.UserContext,
		db,
	)

	router := chi.NewRouter()
	router.Mount("/activities", resource.Router())

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
		router:   router,
	}
}

// =============================================================================
// ACTIVITY CRUD TESTS
// =============================================================================

func TestListActivities_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create test activity
	testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestList-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	assert.NotEmpty(t, data, "Expected at least one activity")
}

func TestListActivities_WithCategoryFilter(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestFilter-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities?category_id=%d", activity.CategoryID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetActivity_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestGet-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.Equal(t, float64(activity.ID), data["id"])
}

func TestGetActivity_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/99999", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestGetActivity_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

func TestCreateActivity_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create a staff member with account for authentication
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Create", "Staff")

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("CreateTest-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"name":             fmt.Sprintf("NewActivity-%d", time.Now().UnixNano()),
		"max_participants": 15,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.TeacherTestClaims(int(account.ID)))

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["id"])

	// Cleanup created activity
}

func TestCreateActivity_BadRequest_MissingName(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("BadReq-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"max_participants": 15,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

func TestCreateActivity_BadRequest_MissingCategoryID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	body := map[string]interface{}{
		"name":             "NoCategoryActivity",
		"max_participants": 15,
		"is_open":          true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

func TestCreateActivity_AllowsNoParticipantLimit(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Unlimited", "Activity")

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("Unlimited-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"name":             "OpenSportsHall",
		"max_participants": nil,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities", body)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.TeacherTestClaims(int(account.ID)))

	require.Equal(t, http.StatusCreated, rr.Code, "body: %s", rr.Body.String())
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Nil(t, data["max_participants"])

	if id, ok := data["id"].(float64); ok {
		activityID := int64(id)
		var persistedCapacity *int
		require.NoError(t, ctx.db.NewRaw(
			`SELECT max_participants FROM activities.groups WHERE id = ?`, activityID,
		).Scan(testpkg.Ctx(t), &persistedCapacity))
		assert.Nil(t, persistedCapacity)
		persistedActivity, err := ctx.services.Activities.GetGroup(testpkg.Ctx(t), activityID)
		require.NoError(t, err)
		assert.Zero(t, persistedActivity.MaxParticipants)
	}
}

func TestUpdateActivity_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Note: CreateTestActivityGroup creates its own staff member as the creator.
	// We use admin permissions to bypass the ownership check.
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("TestUpdate-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"name":             fmt.Sprintf("UpdatedActivity-%d", time.Now().UnixNano()),
		"max_participants": 25,
		"is_open":          false,
		"category_id":      activity.CategoryID,
	}

	// Admin claims carry admin:* which has full access and bypasses ownership checks
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/%d", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateActivity_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create a staff member with account for authentication
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "UpdateNF", "Staff")

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("NotFound-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"name":             "SomeActivity",
		"max_participants": 10,
		"is_open":          true,
		"category_id":      category.ID,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", "/activities/99999", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.TeacherTestClaims(int(account.ID)))

	testutil.AssertNotFound(t, rr)
}

func TestDeleteActivity_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Note: CreateTestActivityGroup creates its own staff member as the creator.
	// We use admin permissions to bypass the ownership check.
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("ToDelete-%d", time.Now().UnixNano()))

	// Admin claims carry admin:* which has full access and bypasses ownership checks
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/%d", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteActivity_NonExistent_ReturnsSuccess(t *testing.T) {
	t.Parallel()
	// Note: With admin permissions, the API allows idempotent deletes - returns success
	// even if the activity doesn't exist. This prevents information disclosure about
	// which activity IDs exist.
	ctx := setupTestContext(t)

	// Use admin claims
	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/activities/99999", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	// With admin permissions, delete is idempotent - returns success even for non-existent
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestDeleteActivity_InvalidID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "DELETE", "/activities/invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// CATEGORY TESTS
// =============================================================================

func TestListCategories_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("TestCat-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/categories", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
	assert.NotEmpty(t, data, "Expected at least one category")
}

func TestGetTimespans_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/timespans", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// SCHEDULE TESTS
// =============================================================================

func TestGetActivitySchedules_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("ScheduleTest-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d/schedules", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestCreateActivitySchedule_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("CreateSched-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"weekday": 1, // Monday
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/schedules", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestCreateActivitySchedule_BadRequest_InvalidWeekday(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("InvalidSched-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"weekday": 10, // Invalid weekday
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/schedules", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

func TestGetAvailableTimeSlots_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/schedules/available", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableTimeSlots_BadRequest_InvalidWeekday(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/schedules/available?weekday=invalid", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// SUPERVISOR TESTS
// =============================================================================

func TestGetActivitySupervisors_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("SupervisorTest-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d/supervisors", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestAssignSupervisor_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("AssignSup-%d", time.Now().UnixNano()))

	staff := testpkg.CreateTestStaff(t, ctx.db, "Supervisor", "Test")

	body := map[string]interface{}{
		"staff_id":   staff.ID,
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/supervisors", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)
}

func TestAssignSupervisor_BadRequest_MissingStaffID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("NoStaff-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"is_primary": true,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/supervisors", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

func TestGetAvailableSupervisors_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	_ = testpkg.CreateTestStaff(t, ctx.db, "Available", "Supervisor")

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities/supervisors/available", nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	_, ok := response["data"].([]interface{})
	require.True(t, ok, "Expected data to be an array")
}

// =============================================================================
// STUDENT ENROLLMENT TESTS
// =============================================================================

func TestGetActivityStudents_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("EnrollTest-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/%d/students", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestEnrollStudent_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("EnrollS-%d", time.Now().UnixNano()))

	student := testpkg.CreateTestStudent(t, ctx.db, "Enroll", "Student", "1a")

	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestEnrollStudent_Conflict_AlreadyEnrolled(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("DupEnroll-%d", time.Now().UnixNano()))

	student := testpkg.CreateTestStudent(t, ctx.db, "Dup", "Enroll", "1a")

	// First enrollment
	req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
	require.Equal(t, http.StatusOK, rr.Code, "First enrollment failed: %s", rr.Body.String())

	// Second enrollment - should conflict
	req = testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil)
	rr = testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	assert.Equal(t, http.StatusConflict, rr.Code, "Expected conflict for duplicate enrollment. Body: %s", rr.Body.String())
}

func TestGetStudentEnrollments_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "GetEnroll", "Student", "1a")

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/students/%d", student.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableActivities_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Available", "Student", "1a")

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/students/%d/available", student.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableActivities_GraduatedStudentNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Available", "Graduate", "4a")

	_, err := ctx.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	req := testutil.NewAuthenticatedRequest(t, "GET", fmt.Sprintf("/activities/students/%d/available", student.ID), nil)
	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestUnenrollStudent_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("Unenroll-%d", time.Now().UnixNano()))

	student := testpkg.CreateTestStudent(t, ctx.db, "Unenroll", "Student", "1a")

	// First enroll the student
	enrollReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil)
	enrollRr := testutil.ExecuteWithAuth(t, ctx.router, enrollReq, testutil.DefaultTestClaims())
	require.Equal(t, http.StatusOK, enrollRr.Code, "Enrollment failed: %s", enrollRr.Body.String())

	// Now unenroll
	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUnenrollStudent_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UnenrollNF-%d", time.Now().UnixNano()))

	student := testpkg.CreateTestStudent(t, ctx.db, "NotEnrolled", "Student", "1a")

	req := testutil.NewAuthenticatedRequest(t, "DELETE", fmt.Sprintf("/activities/%d/students/%d", activity.ID, student.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestBatchEnrollment_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("BatchTest-%d", time.Now().UnixNano()))

	student1 := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Student1", "1a")
	student2 := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Student2", "1b")

	body := map[string]interface{}{
		"student_ids": []int64{student1.ID, student2.ID},
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/%d/students", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestBatchEnrollment_BadRequest_MissingStudentIDs(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("BatchBad-%d", time.Now().UnixNano()))

	body := map[string]interface{}{}

	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/activities/%d/students", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// QUICK CREATE TESTS
// =============================================================================

func TestQuickCreateActivity_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	// Create a staff member with account for authentication
	_, account := testpkg.CreateTestStaffWithAccount(t, ctx.db, "Quick", "Staff")

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("QuickCreate-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"name":             fmt.Sprintf("QuickActivity-%d", time.Now().UnixNano()),
		"category_id":      category.ID,
		"max_participants": 10,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/quick-create", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.TeacherTestClaims(int(account.ID)))

	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok, "Expected data to be an object")
	assert.NotZero(t, data["activity_id"])

	// Cleanup created activity
}

func TestQuickCreateActivity_BadRequest_MissingName(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	category := testpkg.CreateTestActivityCategory(t, ctx.db, fmt.Sprintf("QuickBad-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"category_id":      category.ID,
		"max_participants": 10,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/quick-create", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

func TestQuickCreateActivity_BadRequest_MissingCategoryID(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	body := map[string]interface{}{
		"name":             "NoCategory",
		"max_participants": 10,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/activities/quick-create", body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertBadRequest(t, rr)
}

// =============================================================================
// ADDITIONAL SCHEDULE TESTS (0% coverage functions)
// =============================================================================

func TestGetActivitySchedule_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("GetSched-%d", time.Now().UnixNano()))

	// Create a schedule first using the service
	actSvc := ctx.services.Activities
	schedData := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         1, // Monday
	}
	schedule, err := actSvc.AddSchedule(testpkg.Ctx(t), activity.ID, schedData)
	require.NoError(t, err)
	require.NotNil(t, schedule)

	req := testutil.NewAuthenticatedRequest(t, "GET",
		fmt.Sprintf("/activities/%d/schedules/%d", activity.ID, schedule.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetActivitySchedule_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("GetSchedNF-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "GET",
		fmt.Sprintf("/activities/%d/schedules/999999", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestUpdateActivitySchedule_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSched-%d", time.Now().UnixNano()))

	// Create a schedule first
	actSvc := ctx.services.Activities
	schedData := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         1, // Monday
	}
	schedule, err := actSvc.AddSchedule(testpkg.Ctx(t), activity.ID, schedData)
	require.NoError(t, err)

	body := map[string]interface{}{
		"weekday": 2, // Tuesday
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/schedules/%d", activity.ID, schedule.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateActivitySchedule_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSchedNF-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"weekday": 2,
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/schedules/999999", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestDeleteActivitySchedule_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("DelSched-%d", time.Now().UnixNano()))

	// Create a schedule first
	actSvc := ctx.services.Activities
	schedData := &activities.Schedule{
		ActivityGroupID: activity.ID,
		Weekday:         1, // Monday
	}
	schedule, err := actSvc.AddSchedule(testpkg.Ctx(t), activity.ID, schedData)
	require.NoError(t, err)

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/schedules/%d", activity.ID, schedule.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	// Should succeed with 200 or 204
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNoContent,
		"Expected 200 or 204, got %d", rr.Code)
}

func TestDeleteActivitySchedule_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("DelSchedNF-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/schedules/999999", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

// =============================================================================
// ADDITIONAL SUPERVISOR TESTS (0% coverage functions)
// =============================================================================

func TestUpdateSupervisorRole_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSupRole-%d", time.Now().UnixNano()))
	staff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("SupRole-%d", time.Now().UnixNano()), "Test")

	// Assign supervisor first - get the supervisor record
	actSvc := ctx.services.Activities
	supervisor, err := actSvc.AddSupervisor(testpkg.Ctx(t), activity.ID, staff.ID, false) // false = not primary
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	body := map[string]interface{}{
		"is_primary": true, // Use is_primary instead of role
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/supervisors/%d", activity.ID, supervisor.ID), body) // Use supervisor.ID

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestUpdateSupervisorRole_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("UpdSupRoleNF-%d", time.Now().UnixNano()))

	body := map[string]interface{}{
		"role": "primary",
	}

	req := testutil.NewAuthenticatedRequest(t, "PUT",
		fmt.Sprintf("/activities/%d/supervisors/999999", activity.ID), body)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestRemoveSupervisor_Success(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSup-%d", time.Now().UnixNano()))
	staff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("RemSup-%d", time.Now().UnixNano()), "Test")

	// Assign supervisor first - get the supervisor record
	actSvc := ctx.services.Activities
	supervisor, err := actSvc.AddSupervisor(testpkg.Ctx(t), activity.ID, staff.ID, false) // false = not primary
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/supervisors/%d", activity.ID, supervisor.ID), nil) // Use supervisor.ID

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	// Should succeed with 200 or 204
	assert.True(t, rr.Code == http.StatusOK || rr.Code == http.StatusNoContent,
		"Expected 200 or 204, got %d: %s", rr.Code, rr.Body.String())
}

func TestRemoveSupervisor_NotFound(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSupNF-%d", time.Now().UnixNano()))

	req := testutil.NewAuthenticatedRequest(t, "DELETE",
		fmt.Sprintf("/activities/%d/supervisors/999999", activity.ID), nil)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	testutil.AssertNotFound(t, rr)
}

func TestRemoveSupervisor_WithReplacement_ReplacesSupervisorAtomically(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSupReplace-%d", time.Now().UnixNano()))
	outgoingStaff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("Outgoing-%d", time.Now().UnixNano()), "Caregiver")
	replacementStaff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("Replacement-%d", time.Now().UnixNano()), "Caregiver")

	supervisor, err := ctx.services.Activities.AddSupervisor(testpkg.Ctx(t), activity.ID, outgoingStaff.ID, false)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	req := testutil.NewAuthenticatedRequest(
		t,
		"DELETE",
		fmt.Sprintf(
			"/activities/%d/supervisors/%d?replacement_staff_id=%d",
			activity.ID,
			supervisor.ID,
			replacementStaff.ID,
		),
		nil,
	)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	supervisors, err := ctx.services.Activities.GetGroupSupervisors(testpkg.Ctx(t), activity.ID)
	require.NoError(t, err)

	staffIDs := make([]int64, 0, len(supervisors))
	for _, updatedSupervisor := range supervisors {
		staffIDs = append(staffIDs, updatedSupervisor.StaffID)
	}

	assert.NotContains(t, staffIDs, outgoingStaff.ID)
	assert.Contains(t, staffIDs, replacementStaff.ID)
}

func TestRemoveSupervisor_WithExistingPrimaryReplacement_PreservesPrimaryLead(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSupExistingPrimary-%d", time.Now().UnixNano()))
	primaryStaff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("Primary-%d", time.Now().UnixNano()), "Lead")
	outgoingStaff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("Outgoing-%d", time.Now().UnixNano()), "Caregiver")
	otherStaff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("Other-%d", time.Now().UnixNano()), "Caregiver")

	primarySupervisor, err := ctx.services.Activities.AddSupervisor(testpkg.Ctx(t), activity.ID, primaryStaff.ID, true)
	require.NoError(t, err)
	require.NotNil(t, primarySupervisor)

	outgoingSupervisor, err := ctx.services.Activities.AddSupervisor(testpkg.Ctx(t), activity.ID, outgoingStaff.ID, false)
	require.NoError(t, err)
	require.NotNil(t, outgoingSupervisor)

	otherSupervisor, err := ctx.services.Activities.AddSupervisor(testpkg.Ctx(t), activity.ID, otherStaff.ID, false)
	require.NoError(t, err)
	require.NotNil(t, otherSupervisor)

	req := testutil.NewAuthenticatedRequest(
		t,
		"DELETE",
		fmt.Sprintf(
			"/activities/%d/supervisors/%d?replacement_staff_id=%d",
			activity.ID,
			outgoingSupervisor.ID,
			primaryStaff.ID,
		),
		nil,
	)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	supervisors, err := ctx.services.Activities.GetGroupSupervisors(testpkg.Ctx(t), activity.ID)
	require.NoError(t, err)
	require.Len(t, supervisors, 2)

	var primaryCount int
	for _, supervisor := range supervisors {
		assert.NotEqual(t, outgoingStaff.ID, supervisor.StaffID)
		if supervisor.IsPrimary {
			primaryCount++
			assert.Equal(t, primaryStaff.ID, supervisor.StaffID)
		}
	}
	assert.Equal(t, 1, primaryCount)
}

func TestRemoveSupervisor_OnlySupervisorRequiresReplacement(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, fmt.Sprintf("RemSupOnly-%d", time.Now().UnixNano()))
	onlyStaff := testpkg.CreateTestStaff(t, ctx.db, fmt.Sprintf("Only-%d", time.Now().UnixNano()), "Caregiver")

	supervisor, err := ctx.services.Activities.AddSupervisor(testpkg.Ctx(t), activity.ID, onlyStaff.ID, true)
	require.NoError(t, err)
	require.NotNil(t, supervisor)

	req := testutil.NewAuthenticatedRequest(
		t,
		"DELETE",
		fmt.Sprintf("/activities/%d/supervisors/%d", activity.ID, supervisor.ID),
		nil,
	)

	rr := testutil.ExecuteWithAuth(t, ctx.router, req, testutil.DefaultTestClaims())

	assert.Equal(t, http.StatusConflict, rr.Code)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED", response["code"])
}
