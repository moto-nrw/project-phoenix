// Package attendance_test tests the IoT attendance API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package attendance_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	attendanceAPI "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/attendance"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/services"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/test/testutil"
)

// testContext holds shared test dependencies.
type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *attendanceAPI.Resource
}

// setupTestContext initializes test database, services, and resource.
func setupTestContext(t *testing.T) *testContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create attendance resource
	resource := attendanceAPI.NewResource(
		svc.Users,
		svc.Active,
		svc.Education,
	)

	return &testContext{
		db:       db,
		services: svc,
		resource: resource,
	}
}

// =============================================================================
// GET ATTENDANCE STATUS TESTS
// =============================================================================

func TestGetAttendanceStatus_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/A1B2C3D4", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAttendanceStatus_MissingRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device")

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	// Request with empty RFID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/", nil,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Chi routing will result in 404 for missing param in URL
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}

func TestGetAttendanceStatus_RFIDNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-2")

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	// Request with non-existent RFID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/NONEXISTENT123", nil,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetAttendanceStatus_Success(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device and student with RFID
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-3")
	student := testpkg.CreateTestStudent(t, ctx.db, "Attendance", "Status", "1a")
	// Create RFID card first, then link to student
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

// =============================================================================
// TOGGLE ATTENDANCE TESTS
// =============================================================================

func TestToggleAttendance_NoDevice(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"rfid":   "A1B2C3D4",
		"action": "confirm",
	}

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestToggleAttendance_InvalidJSON(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-1")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Send invalid JSON body - create request manually
	req := httptest.NewRequest("POST", "/toggle", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	// Add device context
	reqCtx := context.WithValue(req.Context(), device.CtxDevice, testDevice)
	req = req.WithContext(reqCtx)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_MissingRFID(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-2")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_Cancel(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-3")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Cancel action still requires RFID
	body := map[string]interface{}{
		"rfid":   "ANYVALUE",
		"action": "cancel",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
}

func TestToggleAttendance_RFIDNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	device := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-4")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"rfid":   "NONEXISTENT999",
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestToggleAttendance_ConfirmDailyCheckoutMissingDestination(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-5")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Daily checkout without destination should fail validation
	body := map[string]interface{}{
		"rfid":   "TESTRFID123",
		"action": "confirm_daily_checkout",
		// destination is missing
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_ConfirmDailyCheckoutInvalidDestination(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-6")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Daily checkout with invalid destination should fail validation
	invalidDest := "invalid_location"
	body := map[string]interface{}{
		"rfid":        "TESTRFID123",
		"action":      "confirm_daily_checkout",
		"destination": invalidDest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_ConfirmDailyCheckoutEmptyDestination(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-7")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Daily checkout with empty destination should fail validation
	emptyDest := ""
	body := map[string]interface{}{
		"rfid":        "TESTRFID123",
		"action":      "confirm_daily_checkout",
		"destination": emptyDest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_DailyCheckoutRFIDNotFound(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-8")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Daily checkout with non-existent RFID
	dest := "zuhause"
	body := map[string]interface{}{
		"rfid":        "NONEXISTENT_RFID_999",
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestToggleAttendance_NormalToggleRFIDNotAssigned(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-9")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Normal toggle with RFID that isn't assigned to anyone
	body := map[string]interface{}{
		"rfid":   "UNASSIGNED_RFID_123",
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetAttendanceStatus_StudentWithGroup(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test device and student with RFID and group
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-4")

	// Create an education group first
	group := testpkg.CreateTestEducationGroup(t, ctx.db, "Test Class 1a")

	// Create student first
	student := testpkg.CreateTestStudent(t, ctx.db, "GroupTest", "Student", "1a")

	// Assign the group to the student
	_, err := ctx.db.NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr("users.students").
		Set("group_id = ?", group.ID).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err, "Failed to assign group to student")

	// Create RFID card and link to student
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_GROUP001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains group info
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		studentInfo, ok := data["student"].(map[string]interface{})
		if ok {
			groupInfo, hasGroup := studentInfo["group"].(map[string]interface{})
			assert.True(t, hasGroup, "Response should contain group info")
			if hasGroup {
				// Group name includes unique suffix from fixture
				groupName, _ := groupInfo["name"].(string)
				assert.Contains(t, groupName, "Test Class 1a")
			}
		}
	}
}

func TestToggleAttendance_InvalidAction(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-10")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Invalid action should fail validation
	body := map[string]interface{}{
		"rfid":   "TESTRFID123",
		"action": "invalid_action",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_DailyCheckoutNoActiveVisit(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-11")
	student := testpkg.CreateTestStudent(t, ctx.db, "NoVisit", "Student", "2a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NOVISIT001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Daily checkout when student has no active visit
	// The service returns an error when visit not found, which results in 500
	dest := "zuhause"
	body := map[string]interface{}{
		"rfid":        rfidCard.ID,
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Service returns error when visit not found, handler returns 500
	// This tests the error handling path in handleDailyCheckout
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestToggleAttendance_NormalToggleValidStudent(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures - valid student with RFID
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-12")
	student := testpkg.CreateTestStudent(t, ctx.db, "Toggle", "Test", "3a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_TOGGLE001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Normal toggle with valid student - exercises lookupStudent, getStaffIDFromContext
	// Will fail at ToggleStudentAttendance since no active session exists
	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Service will return an error since no activity session is running
	// This still tests the lookupStudent and getStaffIDFromContext paths
	assert.True(t, rr.Code >= 400, "Expected error response, got %d: %s", rr.Code, rr.Body.String())
}

func TestToggleAttendance_NormalToggleWithStaffContext(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-13")
	student := testpkg.CreateTestStudent(t, ctx.db, "StaffToggle", "Test", "3b")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_STAFF001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)
	staff := testpkg.CreateTestStaff(t, ctx.db, "TestStaff", "ForToggle")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Normal toggle with staff context - tests getStaffIDFromContext branch
	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Will fail at ToggleStudentAttendance but tests staff context extraction
	assert.True(t, rr.Code >= 400, "Expected error response, got %d: %s", rr.Code, rr.Body.String())
}

func TestToggleAttendance_DailyCheckoutUnterwegs(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// Create test fixtures
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-14")
	student := testpkg.CreateTestStudent(t, ctx.db, "Unterwegs", "Student", "2b")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_UNTERWEGS001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	// Daily checkout with "unterwegs" destination - tests the other destination branch
	dest := "unterwegs"
	body := map[string]interface{}{
		"rfid":        rfidCard.ID,
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Will fail at GetStudentCurrentVisit since no active visit
	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
}

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	require.NotNil(t, router, "Router should return a valid chi.Router")
}

// NOTE: Full success paths for toggleAttendance and confirm_daily_checkout require
// complex staff context setup and active visits/groups. These scenarios are better
// covered by Bruno API tests which have full authentication context and real workflow setup.
