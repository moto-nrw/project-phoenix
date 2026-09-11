package checkin_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// attendanceKiosk is one composed attendance route with its database.
type attendanceKiosk struct {
	kiosk
	attendance *checkinAPI.AttendanceResource
}

func setupAttendanceRoute(t *testing.T) *attendanceKiosk {
	t.Helper()
	db, module := testutil.SetupCheckinModule(t)
	return &attendanceKiosk{
		kiosk:      kiosk{db: db, resource: checkinAPI.NewResource(module.DeviceScan, testRuntime(), nil)},
		attendance: checkinAPI.NewAttendanceResource(module.DeviceScan, testRuntime(), nil),
	}
}

func (k *attendanceKiosk) call(t *testing.T, method, path string, body any, opts ...testutil.RequestOption) *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewTenantRouter(k.db)
	router.Mount("/", k.attendance.Router())
	return testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, method, path, body, opts...))
}

func (k *attendanceKiosk) todayAttendance(t *testing.T, studentID int64) []studentpresence.Attendance {
	t.Helper()
	today := testpkg.TodayDate().String()
	records, err := k.presence(t).ListAttendance(testpkg.Ctx(t), studentpresence.AttendanceFilter{StudentIDs: []int64{studentID}, FromDate: today, UntilDate: today})
	require.NoError(t, err)
	return records
}

func toggle(tag, action string) map[string]any {
	return map[string]any{"rfid": tag, "action": action}
}

func dailyCheckout(tag, destination string) map[string]any {
	return map[string]any{"rfid": tag, "action": "confirm_daily_checkout", "destination": destination}
}

func TestGetAttendanceStatus(t *testing.T) {
	t.Parallel()
	t.Run("requires a device", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		rr := k.call(t, "GET", "/status/A1B2C3D4", nil)
		assert.Equal(t, 401, rr.Code)
	})
	t.Run("unknown card", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		rr := k.call(t, "GET", "/status/NONEXISTENT123", nil, k.device(t, "attendance-2"))
		testutil.AssertNotFound(t, rr)
		assert.Contains(t, rr.Body.String(), "RFID tag not found")
		assert.NotContains(t, rr.Body.String(), "rfid_tag_not_found", "the attendance routes carry no code")
	})
	t.Run("answers the student with the group", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		group := testpkg.CreateTestEducationGroup(t, k.db, "Test Class 1a")
		tag, studentID, _ := k.studentCard(t, "GroupTest", "Student", "1a")
		testpkg.SetStudentGroup(t, k.db, studentID, &group.ID)

		rr := k.call(t, "GET", "/status/"+tag, nil, k.device(t, "attendance-4"))

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		student, ok := data["student"].(map[string]any)
		require.True(t, ok)
		groupInfo, ok := student["group"].(map[string]any)
		require.True(t, ok, "response should contain group info")
		assert.Contains(t, groupInfo["name"], "Test Class 1a")
		attendance, ok := data["attendance"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "not_checked_in", attendance["status"])
		assert.Equal(t, testpkg.TodayDate().String(), attendance["date"])
	})
	t.Run("a staff card is not a student", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		_, _, personID := k.staff(t, "NotStudent", "Person")
		tag := k.card(t, "NOTSTUDENT", personID)
		rr := k.call(t, "GET", "/status/"+tag, nil, k.device(t, "not-student"))
		testutil.AssertNotFound(t, rr)
		assert.Contains(t, rr.Body.String(), "person is not a student")
	})
}

func TestToggleAttendance_RequestContract(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	device := k.device(t, "toggle-contract")

	assert.Equal(t, 401, k.call(t, "POST", "/toggle", toggle("A1B2C3D4", "confirm")).Code)
	testutil.AssertBadRequest(t, k.call(t, "POST", "/toggle", map[string]any{"action": "confirm"}, device))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/toggle", toggle("TESTRFID123", "invalid_action"), device))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/toggle", toggle("TESTRFID123", "confirm_daily_checkout"), device))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/toggle", dailyCheckout("TESTRFID123", "invalid_location"), device))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/toggle", dailyCheckout("TESTRFID123", ""), device))
	testutil.AssertNotFound(t, k.call(t, "POST", "/toggle", toggle("UNASSIGNED_RFID_123", "confirm"), device))
	testutil.AssertNotFound(t, k.call(t, "POST", "/toggle", dailyCheckout("NONEXISTENT_RFID_999", "zuhause"), device))
	require.NotNil(t, k.attendance.Router())
}

func TestToggleAttendance_InvalidJSON(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	router := testutil.NewTenantRouter(k.db)
	router.Mount("/", k.attendance.Router())
	req := testutil.NewRequest("POST", "/toggle", strings.NewReader("invalid json"), k.device(t, "toggle-json"))

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestToggleAttendance_Cancel(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)

	rr := k.call(t, "POST", "/toggle", toggle("ANYVALUE", "cancel"), k.device(t, "toggle-cancel"))

	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "cancelled", data["action"])
	assert.Equal(t, "Attendance tracking cancelled", data["message"])
	attendance, ok := data["attendance"].(map[string]any)
	require.True(t, ok)
	assert.Nil(t, attendance["date"], "no row: the date stays null")
}

func TestToggleAttendance_ConfirmChecksInAndAttributesStaff(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	staff, staffID, _ := k.staff(t, "TestStaff", "ForToggle")
	device, deviceID := k.deviceRow(t, "toggle-staff")
	_, _, sessionID := k.roomWithSession(t, "Staff confirmation")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	testpkg.CreateTestGroupSupervisor(t, k.db, staffID, sessionID, "supervisor")
	tag, _, _ := k.studentCard(t, "StaffToggle", "Test", "3b")

	rr := k.call(t, "POST", "/toggle", toggle(tag, "confirm"), device, staff, testutil.WithIoTDeviceRequest())

	assert.Equal(t, 200, rr.Code, rr.Body.String())
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_in", data["action"])
	assert.Contains(t, data["message"], "Hallo")
	attendance, ok := data["attendance"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "TestStaff ForToggle", attendance["checked_in_by"])
}

func TestToggleAttendance_ConfirmWithoutStaffChecksIn(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	tag, _, _ := k.studentCard(t, "Toggle", "Test", "3a")
	device, deviceID := k.deviceRow(t, "toggle-valid")
	_, supervisorID, _ := k.staff(t, "Session", "Supervisor")
	_, _, sessionID := k.roomWithSession(t, "Device-only confirmation")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	testpkg.CreateTestGroupSupervisor(t, k.db, supervisorID, sessionID, "supervisor")

	rr := k.call(t, "POST", "/toggle", toggle(tag, "confirm"), device, testutil.WithIoTDeviceRequest())

	assert.Equal(t, 200, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"action":"checked_in"`)
}

func TestToggleAttendance_CheckoutEndsOpenVisit(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	device, deviceID := k.deviceRow(t, "normal-toggle-visit")
	staff, staffID, _ := k.staff(t, "NormalToggle", "Staff2")
	tag, studentID, _ := k.studentCard(t, "NormalToggle", "OpenVisit", "5g")
	_, _, sessionID := k.roomWithSession(t, "Normal Toggle Visit Room")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	testpkg.CreateTestGroupSupervisor(t, k.db, staffID, sessionID, "supervisor")
	checkIn := time.Now().Add(-time.Hour)
	testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, checkIn, nil)
	visit := testpkg.CreateTestVisit(t, k.db, studentID, sessionID, checkIn, nil)

	rr := k.call(t, "POST", "/toggle", toggle(tag, "confirm"), device, staff, testutil.WithIoTDeviceRequest())

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_out", responseData(t, rr.Body.Bytes())["action"])
	ended, err := k.presence(t).FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	require.NotNil(t, ended.ExitTime, "toggle checkout must end the open room visit (#895)")
}

func TestToggleAttendance_ConfirmDeviceWithoutSupervisorIsRejected(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	tag, _, _ := k.studentCard(t, "Sessionless", "Confirmation", "3a")
	rr := k.call(t, "POST", "/toggle", toggle(tag, "confirm"), k.device(t, "sessionless-confirm"), testutil.WithIoTDeviceRequest())
	assert.Equal(t, 500, rr.Code, "preserve the legacy generic-confirm authorization response")
	assert.Contains(t, rr.Body.String(), "device must have an active group with supervisors")
}

func TestToggleAttendance_AlumnusRejected(t *testing.T) {
	t.Parallel()
	k := setupAttendanceRoute(t)
	tag, studentID, _ := k.studentCard(t, "Former", "Alumnus", "4z")
	testpkg.SetStudentStatus(t, k.db, studentID, "alumnus")

	rr := k.call(t, "POST", "/toggle", toggle(tag, "confirm"), k.device(t, "toggle-alumnus"))

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "person is not a student")
}

func TestToggleAttendance_DailyCheckout(t *testing.T) {
	t.Parallel()
	t.Run("without today's attendance", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		tag, _, _ := k.studentCard(t, "NoVisit", "Student", "2a")
		for _, destination := range []string{"zuhause", "unterwegs"} {
			rr := k.call(t, "POST", "/toggle", dailyCheckout(tag, destination), k.device(t, "daily-"+destination))
			testutil.AssertErrorResponse(t, rr, 404)
			assert.Contains(t, rr.Body.String(), "student has no attendance record for today")
		}
	})
	t.Run("zuhause closes the row with the device supervisor", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		device, deviceID := k.deviceRow(t, "daily-zuhause")
		_, staffID, _ := k.staff(t, "Zuhause", "Staff")
		tag, studentID, _ := k.studentCard(t, "Zuhause", "CheckedIn", "5a")
		testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-2*time.Hour), nil)
		_, _, sessionID := k.roomWithSession(t, "Daily Zuhause Room")
		testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
		testpkg.CreateTestGroupSupervisor(t, k.db, staffID, sessionID, "supervisor")
		visit := testpkg.CreateTestVisit(t, k.db, studentID, sessionID, time.Now().Add(-2*time.Hour), nil)

		rr := k.call(t, "POST", "/toggle", dailyCheckout(tag, "zuhause"), device)

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "checked_out_daily", data["action"])
		assert.Contains(t, data["message"], "Tschüss")
		assert.Equal(t, false, data["feedback_enabled"], "feedback is opt-in")
		records := k.todayAttendance(t, studentID)
		require.Len(t, records, 1, "daily checkout closes the existing row")
		require.NotNil(t, records[0].CheckOutTime)
		require.NotNil(t, records[0].CheckedOutBy, "the device supervisor is the checkout principal")
		assert.Equal(t, staffID, *records[0].CheckedOutBy)
		ended, err := k.presence(t).FindVisit(testpkg.Ctx(t), visit.ID)
		require.NoError(t, err)
		require.NotNil(t, ended.ExitTime, "daily checkout ends the open room visit (#895)")
	})
	t.Run("zuhause without a supervisor stays device-attributed", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		device, deviceID := k.deviceRow(t, "daily-no-supervisor")
		_, staffID, _ := k.staff(t, "Zuhause", "CheckInOnly")
		tag, studentID, _ := k.studentCard(t, "Zuhause", "NoSupervisor", "5x")
		testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-2*time.Hour), nil)

		rr := k.call(t, "POST", "/toggle", dailyCheckout(tag, "zuhause"), device)

		testutil.AssertSuccessResponse(t, rr, 200)
		assert.Equal(t, "checked_out_daily", responseData(t, rr.Body.Bytes())["action"])
		records := k.todayAttendance(t, studentID)
		require.Len(t, records, 1)
		require.NotNil(t, records[0].CheckOutTime)
		assert.Nil(t, records[0].CheckedOutBy, "no staff actor is invented")
		require.NotNil(t, records[0].CheckedOutDeviceID)
		assert.Equal(t, deviceID, *records[0].CheckedOutDeviceID)
	})
	t.Run("zuhause when already checked out", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		device, deviceID := k.deviceRow(t, "daily-checked-out")
		staff, staffID, _ := k.staff(t, "Zuhause", "Staff2")
		tag, studentID, _ := k.studentCard(t, "Zuhause", "CheckedOut", "5b")
		checkOut := time.Now().Add(-30 * time.Minute)
		testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-2*time.Hour), &checkOut)

		rr := k.call(t, "POST", "/toggle", dailyCheckout(tag, "zuhause"), device, staff)

		testutil.AssertSuccessResponse(t, rr, 200)
		assert.Equal(t, "checked_out_daily", responseData(t, rr.Body.Bytes())["action"])
	})
	t.Run("unterwegs keeps the attendance", func(t *testing.T) {
		t.Parallel()
		k := setupAttendanceRoute(t)
		device, deviceID := k.deviceRow(t, "daily-unterwegs")
		staff, staffID, _ := k.staff(t, "Unterwegs", "Staff")
		tag, studentID, _ := k.studentCard(t, "Unterwegs", "CheckedIn", "5c")
		testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-2*time.Hour), nil)

		rr := k.call(t, "POST", "/toggle", dailyCheckout(tag, "unterwegs"), device, staff)

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "checked_out", data["action"])
		assert.Equal(t, "Viel Spaß!", data["message"])
		records := k.todayAttendance(t, studentID)
		require.Len(t, records, 1)
		assert.Nil(t, records[0].CheckOutTime)
	})
}
