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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	attendanceAPI "github.com/moto-nrw/project-phoenix/api/iot/attendance"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

	// Daily checkout when student has no attendance record
	// The handler returns 404 when the student was never checked in today
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

	// Student has no attendance record, handler returns 404
	// This tests the error handling path in handleDailyCheckout
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
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

	// Daily checkout with "unterwegs" destination — student has no attendance record
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

	// Student has no attendance record, handler returns 404
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

func TestRouter_ReturnsValidRouter(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	router := ctx.resource.Router()
	require.NotNil(t, router, "Router should return a valid chi.Router")
}

// =============================================================================
// DAILY CHECKOUT SUCCESS PATH TESTS
// =============================================================================

// TestToggleAttendance_DailyCheckoutZuhauseCheckedIn tests the daily checkout with
// destination "zuhause" when the student IS checked in — the ToggleStudentAttendance path.
func TestToggleAttendance_DailyCheckoutZuhauseCheckedIn(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// ARRANGE: Create student with RFID and attendance record (checked_in)
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-zuhause-checkedin-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Zuhause", "CheckedIn", "5a")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Zuhause", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_ZUHAUSE_IN001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	// Create attendance record: checked in, NOT checked out
	checkInTime := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	dest := "zuhause"
	body := map[string]interface{}{
		"rfid":        rfidCard.ID,
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
	)

	// ACT
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: Should succeed — student checked_in + zuhause triggers ToggleStudentAttendance
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains daily checkout action
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		assert.Equal(t, "checked_out_daily", data["action"])
		assert.Contains(t, data["message"], "Tschüss")
	}
}

// TestToggleAttendance_DailyCheckoutZuhauseAlreadyCheckedOut tests the daily checkout with
// destination "zuhause" when the student is already checked out — the skip path.
func TestToggleAttendance_DailyCheckoutZuhauseAlreadyCheckedOut(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// ARRANGE: Create student with RFID and attendance record (already checked out)
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-zuhause-checkedout-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Zuhause", "CheckedOut", "5b")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Zuhause", "Staff2")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_ZUHAUSE_OUT001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	// Create attendance record: checked in AND checked out
	checkInTime := time.Now().Add(-2 * time.Hour)
	checkOutTime := time.Now().Add(-30 * time.Minute)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, &checkOutTime)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	dest := "zuhause"
	body := map[string]interface{}{
		"rfid":        rfidCard.ID,
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
	)

	// ACT
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: Should succeed — student already checked_out, skips toggle
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		assert.Equal(t, "checked_out_daily", data["action"])
		assert.Contains(t, data["message"], "Tschüss")
	}
}

// TestToggleAttendance_DailyCheckoutUnterwegsCheckedIn tests the daily checkout with
// destination "unterwegs" when the student is checked in — no attendance change.
func TestToggleAttendance_DailyCheckoutUnterwegsCheckedIn(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// ARRANGE: Create student with RFID and attendance record (checked_in)
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-unterwegs-checkedin-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Unterwegs", "CheckedIn", "5c")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Unterwegs", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_UNTERWEGS_IN001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	// Create attendance record: checked in, NOT checked out
	checkInTime := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	dest := "unterwegs"
	body := map[string]interface{}{
		"rfid":        rfidCard.ID,
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
	)

	// ACT
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: Should succeed — "unterwegs" skips attendance change, returns "checked_out" action
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response — "unterwegs" returns action="checked_out" with "Viel Spaß!"
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		assert.Equal(t, "checked_out", data["action"])
		assert.Equal(t, "Viel Spaß!", data["message"])
	}
}

// TestToggleAttendance_DailyCheckoutNotCheckedIn tests daily checkout rejection
// when the student has no attendance record (not_checked_in status).
func TestToggleAttendance_DailyCheckoutNotCheckedIn(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// ARRANGE: Create student with RFID but NO attendance record
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-notcheckedin-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "NotCheckedIn", "Daily", "5d")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NOTCHECKEDIN001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	dest := "zuhause"
	body := map[string]interface{}{
		"rfid":        rfidCard.ID,
		"action":      "confirm_daily_checkout",
		"destination": dest,
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	// ACT
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: Should return 404 — student has no attendance record
	testutil.AssertErrorResponse(t, rr, http.StatusNotFound)
}

// TestToggleAttendance_NormalToggleSuccess tests the full success path for normal toggle
// when an active session exists with supervisor access via IoT device context.
func TestToggleAttendance_NormalToggleSuccess(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	// ARRANGE: Create all fixtures needed for a complete toggle
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "normal-toggle-success-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "NormalToggle", "Success", "5e")
	staff := testpkg.CreateTestStaff(t, ctx.db, "NormalToggle", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NORMALTOGGLE001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	// Create active session with supervisor (required for IoT device authorization)
	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "normal-toggle-activity")
	room := testpkg.CreateTestRoom(t, ctx.db, "Normal Toggle Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	// Link device to active group
	_, err := ctx.db.NewUpdate().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Set("device_id = ?", testDevice.ID).
		Where("id = ?", activeGroup.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// Create supervisor for the active group
	testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")

	router := chi.NewRouter()
	router.Post("/toggle", ctx.resource.ToggleAttendanceHandler())

	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	// Must include CtxIsIoTDevice so the service authorizes via device supervisor lookup
	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
		func(r *http.Request) {
			reqCtx := context.WithValue(r.Context(), device.CtxIsIoTDevice, true)
			*r = *r.WithContext(reqCtx)
		},
	)

	// ACT: First toggle should check in
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: Should succeed — full success path with sendToggleResponse
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		assert.Equal(t, "checked_in", data["action"])
		assert.Contains(t, data["message"], "Hallo")
		// Verify student info is present
		studentInfo, _ := data["student"].(map[string]interface{})
		if studentInfo != nil {
			assert.NotEmpty(t, studentInfo["first_name"])
		}
	}
}

// TestToggleAttendance_PersonNotLinkedToRFID tests the path where RFID tag exists
// in the persons table but the person has a nil tag (findStudentByRFID nil check).
func TestToggleAttendance_PersonNotStudent(t *testing.T) {
	ctx := setupTestContext(t)
	defer func() { _ = ctx.db.Close() }()

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "not-student-device")

	// Create a staff member (non-student) with RFID
	staff := testpkg.CreateTestStaff(t, ctx.db, "NotStudent", "Person")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NOTSTUDENT001")
	// Link RFID to staff's person (who is NOT a student)
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, rfidCard.ID)

	router := chi.NewRouter()
	router.Get("/status/{rfid}", ctx.resource.GetAttendanceStatusHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/"+rfidCard.ID, nil,
		testutil.WithDeviceContext(testDevice),
	)

	// ACT
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: Should return 404 — person is not a student
	testutil.AssertNotFound(t, rr)
}

// NOTE: Full success paths for toggleAttendance and confirm_daily_checkout require
// complex staff context setup and active visits/groups. The tests above cover
// these scenarios with real database fixtures.
