// Package data_test tests the IoT data API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package data_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *testpkg.DB
	resource *dataAPI.Resource
}

// setupDataRoute initializes the data route.
func setupDataRoute(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupIoTDataModule(t)

	// Create data resource
	resource := dataAPI.NewResource(
		svc.Directory,
		svc.TagAssignments,
		svc.RoomAvailability,
		testRuntime(),
	)

	return &testContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// GET AVAILABLE TEACHERS TESTS
// =============================================================================

func TestGetAvailableTeachers_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	router := ctx.resource.TeachersRouter()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAvailableTeachers_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-1")

	router := ctx.resource.TeachersRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed even if no teachers exist
	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestGetAvailableTeachers_ReturnsTeacherRosterIndependentOfCaregiverState(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-caregiver")
	linkedTeacher, linkedAccount := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Ada", "Caregiver")
	_, _ = testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Legacy", "Teacher")
	_, _ = testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Unmapped", "Teacher")
	_, inactiveAccount := testpkg.CreateTestTeacherWithAccount(t, ctx.db, "Inactive", "Teacher")

	tenantID := linkedTeacher.GetTenantID()
	testpkg.EnsureAccountTenant(t, ctx.db, linkedAccount.ID, tenantID)
	testpkg.EnsureAccountTenant(t, ctx.db, inactiveAccount.ID, tenantID)

	_, err := ctx.db.ExecContext(
		context.Background(),
		`UPDATE auth.account_tenants
		 SET status = 'inactive', updated_at = NOW()
		 WHERE account_id = ? AND tenant_id = ?`,
		inactiveAccount.ID,
		tenantID,
	)
	require.NoError(t, err)

	router := ctx.resource.TeachersRouter()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok)

	displayNames := make([]string, 0, len(data))
	for _, item := range data {
		teacher, ok := item.(map[string]interface{})
		require.True(t, ok)
		displayName, ok := teacher["display_name"].(string)
		require.True(t, ok)
		displayNames = append(displayNames, displayName)
	}

	assert.Contains(t, displayNames, "Ada Caregiver")
	assert.Contains(t, displayNames, "Legacy Teacher")
	assert.Contains(t, displayNames, "Unmapped Teacher")
	assert.Contains(t, displayNames, "Inactive Teacher")
}

// =============================================================================
// GET TEACHER STUDENTS TESTS
// =============================================================================

func TestGetTeacherStudents_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=1", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetTeacherStudents_NoTeacherIDs_ReturnsAllStudents(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-2")
	student := testpkg.CreateTestStudent(t, ctx.db, "AllVis", "Student", "2c")

	router := ctx.resource.Router()

	// Request without teacher_ids parameter — should return all students
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)

	// Parse response and verify our student is in the list
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	assert.True(t, ok, "data should be an array")
	assert.NotEmpty(t, data, "should return students when teacher_ids is absent")

	// Verify our test student appears in results
	var found bool
	for _, item := range data {
		s, _ := item.(map[string]interface{})
		if int64(s["student_id"].(float64)) == student.ID {
			found = true
			assert.Equal(t, "AllVis", s["first_name"])
			break
		}
	}
	assert.True(t, found, "test student should appear in all-students response")
}

func TestGetTeacherStudents_InvalidTeacherID(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-3")

	router := ctx.resource.Router()

	// Request with invalid teacher ID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=invalid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetTeacherStudents_EmptyTeacherIDs_ReturnsEmptyList(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-4")
	// Create a student to verify it is NOT returned
	testpkg.CreateTestStudent(t, ctx.db, "ShouldNot", "Appear", "5x")

	router := ctx.resource.Router()

	// Explicit empty teacher_ids= (key present, value empty) — must NOT return all students.
	// This distinguishes from the "key absent" case which returns all.
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)

	// Verify the data array is empty — explicit empty filter returns no students
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	assert.True(t, ok, "data should be an array")
	assert.Empty(t, data, "explicit empty teacher_ids= should return empty list, not all students")
}

func TestGetTeacherStudents_NonExistentTeacher(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-5")

	router := ctx.resource.Router()

	// Request with non-existent teacher ID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=99999", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Returns success with empty list for non-existent teacher
	testutil.AssertSuccessResponse(t, rr, 200)
}

// =============================================================================
// GET TEACHER ACTIVITIES TESTS
// =============================================================================

func TestGetTeacherActivities_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetTeacherActivities_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-6")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestGetTeacherActivities_WithOccupancy(t *testing.T) {
	t.Parallel()
	tc := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, tc.db, "data-test-device-occ")
	activityGroup := testpkg.CreateTestActivityGroup(t, tc.db, "occ-test-activity")
	room := testpkg.CreateTestRoom(t, tc.db, "occ-test-room")

	// Create an active session for this activity group
	bgCtx := context.Background()
	activeGroup := testpkg.CreateTestActiveGroup(t, tc.db, activityGroup.ID, room.ID)
	defer func() {
		_, _ = tc.db.NewDelete().
			TableExpr("active.groups").
			Where("id = ?", activeGroup.ID).
			Exec(bgCtx)
	}()

	router := tc.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, 200)

	// Verify the response contains is_occupied field
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].([]interface{})
	require.True(t, ok, "data should be an array")

	// Find our test activity and verify it is occupied
	var foundOccupied bool
	for _, item := range data {
		a, _ := item.(map[string]interface{})
		if int64(a["id"].(float64)) == activityGroup.ID {
			foundOccupied = true
			assert.True(t, a["is_occupied"].(bool), "activity with active session should be occupied")
			break
		}
	}
	assert.True(t, foundOccupied, "test activity should appear in response")
}

// =============================================================================
// GET AVAILABLE ROOMS TESTS
// =============================================================================

func TestGetAvailableRooms_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAvailableRooms_Success(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-7")

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestGetAvailableRooms_WithCapacityFilter(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-8")

	router := ctx.resource.Router()

	// Request with capacity filter
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available?capacity=10", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestGetAvailableRooms_InvalidCapacity(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-9")

	router := ctx.resource.Router()

	// Request with invalid capacity (ignored, treated as 0)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available?capacity=invalid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Invalid capacity is silently ignored
	testutil.AssertSuccessResponse(t, rr, 200)
}

// =============================================================================
// CHECK RFID TAG ASSIGNMENT TESTS
// =============================================================================

func TestCheckRFIDTagAssignment_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	router := ctx.resource.Router()

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/A1B2C3D4", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, 401, rr.Code, "Expected 401 for missing device authentication")
}

func TestCheckRFIDTagAssignment_MissingTagID(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-10")

	router := ctx.resource.Router()

	// Request with empty tagId - Chi routing will 404
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Chi routing will result in 404 for missing param in URL
	assert.Contains(t, []int{400, 404}, rr.Code)
}

func TestCheckRFIDTagAssignment_TagNotAssigned(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-11")

	router := ctx.resource.Router()

	// Request with non-existent tag
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/NONEXISTENT123", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Returns success with assigned=false for non-existent tag
	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestCheckRFIDTagAssignment_AssignedToStudent(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-12")
	student := testpkg.CreateTestStudent(t, ctx.db, "RFID", "Student", "2a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID002")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
}

func TestCheckRFIDTagAssignment_AssignedToStaff(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-13")
	staff := testpkg.CreateTestStaff(t, ctx.db, "RFID", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID003")
	// LinkRFIDToStudent works for any person (staff also have a person_id)
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, rfidCard.ID)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
}

// TestCheckRFIDTagAssignment_GraduatedStudentReadsAsUnassigned covers the P1 fix
// (#405 review): graduation is a soft delete, so a tag left on a graduate would
// still resolve here and show the kiosk a name, class and "assigned" state for a
// child no staff-facing route will act on — the dedicated unassign call 404s
// through the alumnus gate. Graduation now releases the tag itself; for tags
// left over from earlier graduations the lookup must report the bracelet as free
// so it can be handed to a current child.
func TestCheckRFIDTagAssignment_GraduatedStudentReadsAsUnassigned(t *testing.T) {
	t.Parallel()
	ctx := setupDataRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-alumnus")
	student := testpkg.CreateTestStudent(t, ctx.db, "RFID", "Alumnus", "4a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFIDALUM")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	dbCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := ctx.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", "alumnus").
		Where("id = ?", student.ID).
		Exec(dbCtx)
	require.NoError(t, err)

	router := ctx.resource.Router()

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, 200)
	body := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := body["data"].(map[string]interface{})
	require.True(t, ok, "response must carry a data object: %s", rr.Body.String())
	assert.Equal(t, false, data["assigned"], "a graduate's leftover tag must read as free")
	assert.Nil(t, data["person"], "and must not name the departed child")
}
