// Package checkin_test tests the IoT attendance API handlers with hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// They use real services with a test database (no mocks).
package checkin_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// attendanceTestContext holds shared test dependencies.
type attendanceTestContext struct {
	db       *bun.DB
	resource *checkinAPI.AttendanceResource
}

// setupAttendanceRoute initializes the attendance route.
func setupAttendanceRoute(t *testing.T) *attendanceTestContext {
	t.Helper()

	db, svc := testutil.SetupAPITest(t)

	// Create attendance resource
	resource := checkinAPI.NewAttendanceResource(
		svc.Users,
		svc.Active,
		svc.Education,
		nil,
	)

	return &attendanceTestContext{
		db:       db,
		resource: resource,
	}
}

// =============================================================================
// GET ATTENDANCE STATUS TESTS
// =============================================================================

func TestGetAttendanceStatus_NoDevice(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request without device context should return 401
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/A1B2C3D4", nil)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "Expected 401 for missing device authentication")
}

func TestGetAttendanceStatus_MissingRFID(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// Create test device
	device := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request with empty RFID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/", nil,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	// Chi routing will result in 404 for missing param in URL
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusNotFound}, rr.Code)
}

func TestGetAttendanceStatus_RFIDNotFound(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-2")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Request with non-existent RFID
	req := testutil.NewAuthenticatedRequest(t, "GET", "/status/NONEXISTENT123", nil,
		testutil.WithDeviceContext(device),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestGetAttendanceStatus_Success(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// Create test device and student with RFID
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "attendance-test-device-3")
	student := testpkg.CreateTestStudent(t, ctx.db, "Attendance", "Status", "1a")
	// Create RFID card first, then link to student
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-1")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-2")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-3")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	device := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-4")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-5")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-6")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-7")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-8")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-9")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

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

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-10")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// Create test fixtures
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-11")
	student := testpkg.CreateTestStudent(t, ctx.db, "NoVisit", "Student", "2a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NOVISIT001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// Create test fixtures - valid student with RFID
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-12")
	student := testpkg.CreateTestStudent(t, ctx.db, "Toggle", "Test", "3a")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_TOGGLE001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Normal toggle with valid student - exercises lookupStudent, getStaffIDFromContext.
	// Since #2329 the web toggle path no longer requires teacher-group access,
	// so the toggle succeeds and checks the student in.
	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected successful toggle, got %d: %s", rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"action":"checked_in"`)
}

func TestToggleAttendance_NormalToggleWithStaffContext(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// Create test fixtures
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-13")
	student := testpkg.CreateTestStudent(t, ctx.db, "StaffToggle", "Test", "3b")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_STAFF001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)
	staff := testpkg.CreateTestStaff(t, ctx.db, "TestStaff", "ForToggle")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	// Normal toggle with staff context - tests getStaffIDFromContext branch.
	// Since #2329 any staff member may toggle any student, so the toggle
	// succeeds and the check-in is attributed to the staff member.
	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
	)

	rr := testutil.ExecuteRequest(router, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected successful toggle, got %d: %s", rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"checked_in_by":"TestStaff ForToggle"`)
}

func TestToggleAttendance_DailyCheckoutUnterwegs(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// Create test fixtures
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-test-device-14")
	student := testpkg.CreateTestStudent(t, ctx.db, "Unterwegs", "Student", "2b")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_UNTERWEGS001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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

func TestAttendanceRouter_ReturnsValidRouter(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	router := ctx.resource.Router()
	require.NotNil(t, router, "Router should return a valid chi.Router")
}

// =============================================================================
// DAILY CHECKOUT SUCCESS PATH TESTS
// =============================================================================

// TestToggleAttendance_DailyCheckoutZuhauseCheckedIn tests the daily checkout with
// destination "zuhause" when the student is checked in. Daily checkout must use
// the device's active supervisor as checked_out_by; device+PIN alone is not an
// auditable attendance principal.
func TestToggleAttendance_DailyCheckoutZuhauseCheckedIn(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// ARRANGE: Create student with RFID and attendance record (checked_in)
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-zuhause-checkedin-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Zuhause", "CheckedIn", "5a")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Zuhause", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_ZUHAUSE_IN001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	// Create attendance record: checked in, NOT checked out
	checkInTime := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "daily-zuhause-checkedin-activity")
	room := testpkg.CreateTestRoom(t, ctx.db, "Daily Zuhause CheckedIn Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	_, err := ctx.db.NewUpdate().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Set("device_id = ?", testDevice.ID).
		Where("id = ?", activeGroup.ID).
		Exec(context.Background())
	require.NoError(t, err)
	testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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

	// ASSERT: Should succeed — student checked_in + zuhause performs an explicit checkout.
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	// Verify response contains daily checkout action
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	if ok {
		assert.Equal(t, "checked_out_daily", data["action"])
		assert.Contains(t, data["message"], "Tschüss")
	}

	var records []*activeModel.Attendance
	err = ctx.db.NewSelect().
		Model(&records).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, student.ID).
		Where(`"attendance".date = ?`, timezone.TodayDate()).
		Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, records, 1, "daily checkout must close the existing row instead of inserting a new one")
	require.NotNil(t, records[0].CheckOutTime)
	require.NotNil(t, records[0].CheckedOutBy, "IoT daily checkout must write an auditable staff FK")
	assert.Equal(t, staff.ID, *records[0].CheckedOutBy)
}

func TestToggleAttendance_DailyCheckoutZuhauseUsesAuthenticatedDeviceWithoutSupervisor(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-zuhause-no-supervisor-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Zuhause", "NoSupervisor", "5x")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Zuhause", "CheckInOnly")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_ZUHAUSE_NOSUP")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	checkInTime := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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

	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	data, ok := response["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "checked_out_daily", data["action"])

	var records []*activeModel.Attendance
	err := ctx.db.NewSelect().
		Model(&records).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, student.ID).
		Where(`"attendance".date = ?`, timezone.TodayDate()).
		Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.NotNil(t, records[0].CheckOutTime, "device-attributed daily checkout must close attendance")
	assert.Nil(t, records[0].CheckedOutBy, "checkout must not invent a staff actor")
	require.NotNil(t, records[0].CheckedOutDeviceID)
	assert.Equal(t, testDevice.ID, *records[0].CheckedOutDeviceID)
}

// TestToggleAttendance_DailyCheckoutZuhauseAlreadyCheckedOut tests the daily checkout with
// destination "zuhause" when the student is already checked out — the skip path.
func TestToggleAttendance_DailyCheckoutZuhauseAlreadyCheckedOut(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

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

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// ARRANGE: Create student with RFID and attendance record (checked_in)
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-unterwegs-checkedin-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Unterwegs", "CheckedIn", "5c")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Unterwegs", "Staff")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_UNTERWEGS_IN001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	// Create attendance record: checked in, NOT checked out
	checkInTime := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// ARRANGE: Create student with RFID but NO attendance record
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-notcheckedin-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "NotCheckedIn", "Daily", "5d")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NOTCHECKEDIN001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

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

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "not-student-device")

	// Create a staff member (non-student) with RFID
	staff := testpkg.CreateTestStaff(t, ctx.db, "NotStudent", "Person")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NOTSTUDENT001")
	// Link RFID to staff's person (who is NOT a student)
	testpkg.LinkRFIDToStudent(t, ctx.db, staff.PersonID, rfidCard.ID)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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

// =============================================================================
// CHECKOUT ENDS OPEN VISIT TESTS (issue #895)
// =============================================================================

// TestToggleAttendance_DailyCheckoutZuhause_EndsOpenVisit verifies that the
// daily checkout ("zuhause") not only closes the attendance row but also ends
// a still-open room visit in the same request. Before issue #895 was fixed,
// the visit stayed open and deadlocked the student (checkin 409 / checkout 404).
func TestToggleAttendance_DailyCheckoutZuhause_EndsOpenVisit(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// ARRANGE: checked-in student with RFID and a still-open room visit
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "daily-zuhause-visit-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Zuhause", "OpenVisit", "5f")
	staff := testpkg.CreateTestStaff(t, ctx.db, "Zuhause", "Staff3")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_ZUHAUSE_VISIT01")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	checkInTime := time.Now().Add(-2 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "daily-zuhause-visit-activity")
	room := testpkg.CreateTestRoom(t, ctx.db, "Daily Zuhause Visit Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)
	_, err := ctx.db.NewUpdate().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Set("device_id = ?", testDevice.ID).
		Where("id = ?", activeGroup.ID).
		Exec(context.Background())
	require.NoError(t, err)
	testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, checkInTime, nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

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

	// ASSERT: success, attendance closed AND visit ended
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var records []*activeModel.Attendance
	err = ctx.db.NewSelect().
		Model(&records).
		ModelTableExpr(`active.attendance AS "attendance"`).
		Where(`"attendance".student_id = ?`, student.ID).
		Where(`"attendance".date = ?`, timezone.TodayDate()).
		Scan(context.Background())
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.NotNil(t, records[0].CheckOutTime, "daily checkout must close the attendance row")
	require.NotNil(t, records[0].CheckedOutBy, "daily checkout must write the device supervisor as checkout principal")
	assert.Equal(t, staff.ID, *records[0].CheckedOutBy)

	endedVisit := new(activeModel.Visit)
	err = ctx.db.NewSelect().
		Model(endedVisit).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".id = ?`, visit.ID).
		Scan(context.Background())
	require.NoError(t, err)
	require.NotNil(t, endedVisit.ExitTime, "daily checkout must end the open room visit (issue #895)")
}

// TestToggleAttendance_NormalToggle_CheckoutEndsOpenVisit verifies that the
// normal kiosk toggle's checkout branch also ends a still-open room visit.
func TestToggleAttendance_NormalToggle_CheckoutEndsOpenVisit(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	// ARRANGE: full toggle setup (device-linked session + supervisor) with a
	// checked-in student who still has an open visit in the session's room.
	testDevice := testpkg.CreateTestDevice(t, ctx.db, "normal-toggle-visit-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "NormalToggle", "OpenVisit", "5g")
	staff := testpkg.CreateTestStaff(t, ctx.db, "NormalToggle", "Staff2")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_NORMALTOGGLE02")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	activity := testpkg.CreateTestActivityGroup(t, ctx.db, "normal-toggle-visit-activity")
	room := testpkg.CreateTestRoom(t, ctx.db, "Normal Toggle Visit Room")
	activeGroup := testpkg.CreateTestActiveGroup(t, ctx.db, activity.ID, room.ID)

	_, err := ctx.db.NewUpdate().
		Model(activeGroup).
		ModelTableExpr(`active.groups`).
		Set("device_id = ?", testDevice.ID).
		Where("id = ?", activeGroup.ID).
		Exec(context.Background())
	require.NoError(t, err)

	testpkg.CreateTestGroupSupervisor(t, ctx.db, staff.ID, activeGroup.ID, "supervisor")

	checkInTime := time.Now().Add(-1 * time.Hour)
	testpkg.CreateTestAttendance(t, ctx.db, student.ID, staff.ID, testDevice.ID, checkInTime, nil)
	visit := testpkg.CreateTestVisit(t, ctx.db, student.ID, activeGroup.ID, checkInTime, nil)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
		testutil.WithStaffContext(staff),
		func(r *http.Request) {
			reqCtx := context.WithValue(r.Context(), device.CtxIsIoTDevice, true)
			*r = *r.WithContext(reqCtx)
		},
	)

	// ACT: student is checked_in, so the toggle performs a checkout
	rr := testutil.ExecuteRequest(router, req)

	// ASSERT: success with checked_out action AND the visit is ended
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	if data, ok := response["data"].(map[string]interface{}); ok {
		assert.Equal(t, "checked_out", data["action"])
	}

	endedVisit := new(activeModel.Visit)
	err = ctx.db.NewSelect().
		Model(endedVisit).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".id = ?`, visit.ID).
		Scan(context.Background())
	require.NoError(t, err)
	require.NotNil(t, endedVisit.ExitTime, "toggle checkout must end the open room visit (issue #895)")
}

// TestToggleAttendance_AlumnusRejected: a graduated (alumnus) student's RFID
// card must not check in anymore. The scan resolves person→student, but the
// alumnus status makes the student invisible to the kiosk — same wire error
// as "person is not a student" so PyrePortal needs no new mapping.
func TestToggleAttendance_AlumnusRejected(t *testing.T) {
	t.Parallel()
	ctx := setupAttendanceRoute(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "toggle-alumnus-device")
	student := testpkg.CreateTestStudent(t, ctx.db, "Former", "Alumnus", "4z")
	rfidCard := testpkg.CreateTestRFIDCard(t, ctx.db, "TESTRFID_ALUM001")
	testpkg.LinkRFIDToStudent(t, ctx.db, student.PersonID, rfidCard.ID)

	_, err := ctx.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(users.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	router := testutil.NewTenantRouter(ctx.db)
	router.Mount("/", ctx.resource.Router())

	body := map[string]interface{}{
		"rfid":   rfidCard.ID,
		"action": "confirm",
	}

	req := testutil.NewAuthenticatedRequest(t, "POST", "/toggle", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}
