// Package data_test tests the IoT data API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package data_test

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"

	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *dataAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create data resource
	resource := dataAPI.NewResource(
		svc.IoT,
		svc.Users,
		svc.Activities,
		svc.Facilities,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// GET AVAILABLE TEACHERS TESTS
// =============================================================================

func TestGetAvailableTeachers_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/teachers", ctx.resource.GetAvailableTeachersHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/teachers", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAvailableTeachers_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-1")

	router := chi.NewRouter()
	router.Get("/teachers", ctx.resource.GetAvailableTeachersHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/teachers", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should succeed even if no teachers exist
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET TEACHER STUDENTS TESTS
// =============================================================================

func TestGetTeacherStudents_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/students", ctx.resource.GetTeacherStudentsHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=1", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetTeacherStudents_NoTeacherIDs(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-2")

	router := chi.NewRouter()
	router.Get("/students", ctx.resource.GetTeacherStudentsHandler())

	// Request without teacher_ids parameter
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Should return success with empty list (not an error)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetTeacherStudents_InvalidTeacherID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-3")

	router := chi.NewRouter()
	router.Get("/students", ctx.resource.GetTeacherStudentsHandler())

	// Request with invalid teacher ID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=invalid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestGetTeacherStudents_EmptyTeacherIDs(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-4")

	router := chi.NewRouter()
	router.Get("/students", ctx.resource.GetTeacherStudentsHandler())

	// Request with empty teacher_ids parameter
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Returns success with empty list when no valid IDs
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetTeacherStudents_NonExistentTeacher(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-5")

	router := chi.NewRouter()
	router.Get("/students", ctx.resource.GetTeacherStudentsHandler())

	// Request with non-existent teacher ID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids=99999", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Returns success with empty list for non-existent teacher
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET TEACHER ACTIVITIES TESTS
// =============================================================================

func TestGetTeacherActivities_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/activities", ctx.resource.GetTeacherActivitiesHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetTeacherActivities_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-6")

	router := chi.NewRouter()
	router.Get("/activities", ctx.resource.GetTeacherActivitiesHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/activities", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// GET AVAILABLE ROOMS TESTS
// =============================================================================

func TestGetAvailableRooms_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/rooms/available", ctx.resource.GetAvailableRoomsHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAvailableRooms_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-7")

	router := chi.NewRouter()
	router.Get("/rooms/available", ctx.resource.GetAvailableRoomsHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableRooms_WithCapacityFilter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-8")

	router := chi.NewRouter()
	router.Get("/rooms/available", ctx.resource.GetAvailableRoomsHandler())

	// Request with capacity filter
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available?capacity=10", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestGetAvailableRooms_InvalidCapacity(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-9")

	router := chi.NewRouter()
	router.Get("/rooms/available", ctx.resource.GetAvailableRoomsHandler())

	// Request with invalid capacity (ignored, treated as 0)
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rooms/available?capacity=invalid", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Invalid capacity is silently ignored
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// CHECK RFID TAG ASSIGNMENT TESTS
// =============================================================================

func TestCheckRFIDTagAssignment_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/rfid/{tagId}", ctx.resource.CheckRFIDTagAssignmentHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/A1B2C3D4", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestCheckRFIDTagAssignment_MissingTagID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-10")

	router := chi.NewRouter()
	router.Get("/rfid/{tagId}", ctx.resource.CheckRFIDTagAssignmentHandler())

	// Request with empty tagId - Chi routing will 404
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Chi routing will result in 404 for missing param in URL
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}

func TestCheckRFIDTagAssignment_TagNotAssigned(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-11")

	router := chi.NewRouter()
	router.Get("/rfid/{tagId}", ctx.resource.CheckRFIDTagAssignmentHandler())

	// Request with non-existent tag
	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/NONEXISTENT123", nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Returns success with assigned=false for non-existent tag
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestCheckRFIDTagAssignment_AssignedToStudent(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-12")
	student := testpkg.CreateTestStudent(t, ctx.db, "RFID", "Student", "2a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID002")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Get("/rfid/{tagId}", ctx.resource.CheckRFIDTagAssignmentHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestCheckRFIDTagAssignment_AssignedToStaff(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "data-test-device-13")
	staff := testpkg.CreateTestStaff(t, ctx.db, "RFID", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID003")
	// LinkRFIDToStudent works for any person (staff also have a person_id)
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Get("/rfid/{tagId}", ctx.resource.CheckRFIDTagAssignmentHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/rfid/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}
