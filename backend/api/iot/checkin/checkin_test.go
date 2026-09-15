// Package checkin_test exercises the kiosk scan HTTP boundary with the real
// device-scan workflow composed over the retained services, repositories and
// tenant-scoped transactions. The resources are composed over the
// test-support envelope, so this package needs neither the shared HTTP
// package nor the persistence models (#2698).
package checkin_test

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// kiosk is one composed check-in route with its database.
type kiosk struct {
	db       *testpkg.DB
	resource *checkinAPI.Resource
}

func testRuntime() checkinAPI.Runtime {
	return checkinAPI.Runtime{Success: testutil.RespondSuccess, Failure: testutil.RespondCoded}
}

// setupCheckinRoute composes the production check-in resource. An optional
// clock pins the instant the scans are admitted.
func setupCheckinRoute(t *testing.T, clocks ...func() time.Time) *kiosk {
	t.Helper()
	db, module := testutil.SetupCheckinModule(t, clocks...)
	return &kiosk{db: db, resource: checkinAPI.NewResource(module.DeviceScan, testRuntime(), nil)}
}

// call performs one device-authenticated request through the tenant
// transaction middleware.
func (k *kiosk) call(t *testing.T, method, path string, body any, opts ...testutil.RequestOption) *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewTenantRouter(k.db)
	router.Mount("/", k.resource.Router())
	return testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, method, path, body, opts...))
}

// callIsolated performs a request against the test's own tenant runtime,
// for scans that provision rooms and activities.
func (k *kiosk) callIsolated(t *testing.T, method, path string, body any, opts ...testutil.RequestOption) *httptest.ResponseRecorder {
	t.Helper()
	router := testutil.NewTenantRouter(k.db)
	router.Mount("/", k.resource.Router())
	return testutil.ExecuteRequestForTest(t, router, testutil.NewAuthenticatedRequest(t, method, path, body, opts...))
}

// device creates a kiosk device that counts as online.
func (k *kiosk) device(t *testing.T, name string) testutil.RequestOption {
	t.Helper()
	device := testpkg.CreateTestDevice(t, k.db, name)
	now := time.Now()
	device.LastSeen = &now
	return testutil.WithDeviceContext(device)
}

// deviceRow creates a kiosk device and returns its principal option and id.
func (k *kiosk) deviceRow(t *testing.T, name string) (testutil.RequestOption, int64) {
	t.Helper()
	device := testpkg.CreateTestDevice(t, k.db, name)
	now := time.Now()
	device.LastSeen = &now
	return testutil.WithDeviceContext(device), device.ID
}

// staff creates a staff member and returns its principal option and ids.
func (k *kiosk) staff(t *testing.T, first, last string) (testutil.RequestOption, int64, int64) {
	t.Helper()
	staff := testpkg.CreateTestStaff(t, k.db, first, last)
	return testutil.WithStaffContext(staff), staff.ID, staff.PersonID
}

// studentCard creates a student and links a fresh card; it returns the
// card tag with the student and person ids.
func (k *kiosk) studentCard(t *testing.T, first, last, class string) (tag string, studentID, personID int64) {
	t.Helper()
	student := testpkg.CreateTestStudent(t, k.db, first, last, class)
	card := testpkg.CreateTestRFIDCard(t, k.db, fmt.Sprintf("TAG%d", time.Now().UnixNano()))
	testpkg.LinkRFIDToStudent(t, k.db, student.PersonID, card.ID)
	return card.ID, student.ID, student.PersonID
}

// card links a fresh card to a person.
func (k *kiosk) card(t *testing.T, prefix string, personID int64) string {
	t.Helper()
	card := testpkg.CreateTestRFIDCard(t, k.db, fmt.Sprintf("%s%d", strings.ToUpper(prefix), time.Now().UnixNano()))
	testpkg.LinkRFIDToStudent(t, k.db, personID, card.ID)
	return card.ID
}

// roomWithSession creates a room with a running session; the returned ids
// are room, activity and session.
func (k *kiosk) roomWithSession(t *testing.T, name string) (roomID, activityID, sessionID int64) {
	t.Helper()
	room := testpkg.CreateTestRoom(t, k.db, name)
	activity := testpkg.CreateTestActivityGroup(t, k.db, name+" Activity")
	session := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, room.ID)
	return room.ID, activity.ID, session.ID
}

func (k *kiosk) presence(t *testing.T) *studentpresence.Module {
	t.Helper()
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: k.db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	return module
}

func (k *kiosk) openVisits(t *testing.T, studentID int64) []studentpresence.Visit {
	t.Helper()
	visits, err := k.presence(t).ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{studentID}})
	require.NoError(t, err)
	return visits
}

func responseData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	response := testutil.ParseJSONResponse(t, body)
	data, ok := response["data"].(map[string]any)
	require.True(t, ok, "response should have a data object: %s", body)
	return data
}

func checkinBody(tag string, roomID int64) map[string]any {
	return map[string]any{"student_rfid": tag, "action": "checkin", "room_id": roomID}
}

func checkoutBody(tag string) map[string]any {
	return map[string]any{"student_rfid": tag, "action": "checkout"}
}

// ---------------------------------------------------------------------------
// Ping and status
// ---------------------------------------------------------------------------

func TestDevicePing_Success(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)

	rr := k.call(t, "POST", "/ping", nil, k.device(t, "ping-test"))

	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	for _, key := range []string{"device_id", "status", "is_online", "ping_time", "session_active"} {
		assert.Contains(t, data, key)
	}
	assert.Equal(t, false, data["session_active"])
}

func TestDevicePing_ReportsRunningSession(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "ping-session")
	_, _, sessionID := k.roomWithSession(t, "Ping Room")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	before := testpkg.ActiveGroupLastActivity(t, k.db, sessionID)
	time.Sleep(10 * time.Millisecond)

	rr := k.call(t, "POST", "/ping", nil, device)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, true, responseData(t, rr.Body.Bytes())["session_active"])
	assert.False(t, testpkg.ActiveGroupLastActivity(t, k.db, sessionID).Before(before), "the ping keeps the session alive")
}

func TestDevicePing_Unauthorized(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)

	rr := k.call(t, "POST", "/ping", nil)

	testutil.AssertUnauthorized(t, rr)
	assert.JSONEq(t, `{"status":"error","error":"device API key is required"}`, rr.Body.String())
}

func TestDeviceStatus(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)

	rr := k.call(t, "GET", "/status", nil, k.device(t, "status-test"))
	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	device, ok := data["device"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, device["is_active"])
	assert.Contains(t, data, "authenticated_at")

	testutil.AssertUnauthorized(t, k.call(t, "GET", "/status", nil))
}

// ---------------------------------------------------------------------------
// Request contract
// ---------------------------------------------------------------------------

func TestDeviceCheckin_RequestValidation(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	device := k.device(t, "validation")

	testutil.AssertUnauthorized(t, k.call(t, "POST", "/checkin", map[string]any{"student_rfid": "12345"}))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/checkin", map[string]any{"action": "checkin"}, device))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/checkin", map[string]any{"student_rfid": ""}, device))
	testutil.AssertBadRequest(t, k.call(t, "POST", "/checkin", map[string]any{"student_rfid": "test-rfid", "action": "invalid"}, device))
	rr := k.call(t, "POST", "/checkin", map[string]any{"student_rfid": 12345}, device)
	assert.Contains(t, []int{400, 404}, rr.Code)
}

func TestRouter_EndpointsRequireDevice(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	router := k.resource.Router()
	require.NotNil(t, router)
	for _, route := range []struct{ method, path string }{{"POST", "/checkin"}, {"POST", "/ping"}, {"GET", "/status"}, {"POST", "/pickup-query"}} {
		rr := testutil.ExecuteRequest(router, testutil.NewAuthenticatedRequest(t, route.method, route.path, nil))
		assert.Equal(t, 401, rr.Code, route.path)
	}
}

// ---------------------------------------------------------------------------
// Student scans
// ---------------------------------------------------------------------------

func TestDeviceCheckin_UnknownCardIsNotFoundWithCode(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)

	rr := k.call(t, "POST", "/checkin", map[string]any{"student_rfid": "nonexistent-rfid-tag", "action": "checkin"}, k.device(t, "not-found"))

	testutil.AssertNotFound(t, rr)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "RFID tag not found", response["error"])
	assert.Equal(t, "rfid_tag_not_found", response["code"])
}

func TestDeviceCheckin_NoSessionInRoom(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	tag, _, _ := k.studentCard(t, "CheckIn", "Student", "1a")
	room := testpkg.CreateTestRoom(t, k.db, "Checkin Room")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, room.ID), k.device(t, "no-groups"))

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "no active groups in specified room")
}

func TestDeviceCheckin_SuccessfulCheckin(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Checkin", "Staff")
	tag, studentID, _ := k.studentCard(t, "Success", "Checkin", "1a")
	roomID, _, _ := k.roomWithSession(t, "Success Room")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomID), k.device(t, "success"), staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "Hallo Success!", data["message"])
	assert.Equal(t, false, data["daily_checkout_available"])
	assert.Equal(t, false, data["feedback_enabled"])
	assert.Contains(t, data, "visit_id")
	assert.Len(t, k.openVisits(t, studentID), 1)
}

func TestDeviceCheckin_CheckoutWithActiveVisit(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	tag, studentID, _ := k.studentCard(t, "Checkout", "Student", "1a")
	roomID, _, sessionID := k.roomWithSession(t, "Checkout Test Room")
	room := testpkg.CreateTestRoom(t, k.db, "unused")
	_ = room
	visit := testpkg.CreateTestVisit(t, k.db, studentID, sessionID, time.Now().Add(-15*time.Minute), nil)

	rr := k.call(t, "POST", "/checkin", checkoutBody(tag), k.device(t, "checkout-test"))

	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_out", data["action"])
	assert.Equal(t, "success", data["status"])
	assert.Equal(t, "Checkout Student", data["student_name"])
	assert.EqualValues(t, visit.ID, data["visit_id"])
	ended, err := k.presence(t).FindVisit(testpkg.Ctx(t), visit.ID)
	require.NoError(t, err)
	require.NotNil(t, ended.ExitTime)
	_ = roomID
}

func TestDeviceCheckin_CheckoutWithoutActiveVisit(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	tag, _, _ := k.studentCard(t, "NoVisit", "Test", "1a")

	rr := k.call(t, "POST", "/checkin", checkoutBody(tag), k.device(t, "checkout-no-visit"))

	testutil.AssertBadRequest(t, rr)
	assert.Contains(t, rr.Body.String(), "room_id is required for check-in")
}

func TestDeviceCheckin_RoomTransfer(t *testing.T) {
	t.Parallel()
	t.Run("succeeds into a room with a session", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		staff, _, _ := k.staff(t, "Transfer", "Staff")
		tag, studentID, _ := k.studentCard(t, "Transfer", "Test", "2b")
		_, _, sessionA := k.roomWithSession(t, "Room A")
		roomB, _, _ := k.roomWithSession(t, "Room B")
		testpkg.CreateTestVisit(t, k.db, studentID, sessionA, time.Now().Add(-10*time.Minute), nil)

		rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomB), k.device(t, "transfer"), staff)

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "transferred", data["action"])
		assert.Contains(t, data, "previous_room")
		assert.Contains(t, data["message"], "Gewechselt von")
	})
	t.Run("fails into a room without a session and preserves the source visit", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		tag, studentID, _ := k.studentCard(t, "Transfer", "Student", "3c")
		_, _, sessionA := k.roomWithSession(t, "Transfer Room 1")
		roomB := testpkg.CreateTestRoom(t, k.db, "Transfer Room 2")
		visit := testpkg.CreateTestVisit(t, k.db, studentID, sessionA, time.Now(), nil)

		rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomB.ID), k.device(t, "transfer-invalid"), testutil.WithIoTDeviceRequest())

		testutil.AssertNotFound(t, rr)
		assert.Contains(t, rr.Body.String(), "no active groups in specified room")
		persisted, err := k.presence(t).FindVisit(testpkg.Ctx(t), visit.ID)
		require.NoError(t, err)
		assert.Nil(t, persisted.ExitTime, "a rejected transfer must retain the source room")
	})
}

func TestDeviceCheckin_SameRoomScanChecksOut(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Same", "Room")
	tag, studentID, _ := k.studentCard(t, "Same", "RoomStudent", "2a")
	roomID, _, sessionID := k.roomWithSession(t, "Same Room Test")
	testpkg.CreateTestVisit(t, k.db, studentID, sessionID, time.Now().Add(-5*time.Minute), nil)

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomID), k.device(t, "same-room"), staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_out", responseData(t, rr.Body.Bytes())["action"])
}

func TestDeviceCheckin_OtherSchoolsCardIsInvisible(t *testing.T) {
	t.Parallel()
	// A device of another school runs under that school's RLS: this
	// school's card is unknown there, and the scan answers "not found"
	// without the child's data.
	k := setupCheckinRoute(t)
	tag, _, _ := k.studentCard(t, "Isolated", "Child", "1a")
	roomID, _, _ := k.roomWithSession(t, "Isolated Room")
	otherTenant, _ := testpkg.CreateTestTenant(t, k.db)
	otherDevice := testpkg.CreateTestDeviceForTenant(t, k.db, otherTenant, fmt.Sprintf("other-%d", time.Now().UnixNano()))

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomID), testutil.WithDeviceContext(otherDevice))

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "RFID tag not found")
	assert.NotContains(t, rr.Body.String(), "Isolated")
}

// ---------------------------------------------------------------------------
// Staff scans
// ---------------------------------------------------------------------------

func TestDeviceCheckin_SupervisorRFIDAuthentication(t *testing.T) {
	t.Parallel()
	t.Run("authenticates supervisor with active session", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		device, deviceID := k.deviceRow(t, "staff-rfid")
		_, _, personID := k.staff(t, "Staff", "Member")
		tag := k.card(t, "STAFF", personID)
		_, _, sessionID := k.roomWithSession(t, "Supervisor Room")
		testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)

		rr := k.call(t, "POST", "/checkin", map[string]any{"student_rfid": tag, "action": "checkin"}, device)

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "supervisor_authenticated", data["action"])
		assert.Contains(t, data["student_name"], "Staff")
		assert.Contains(t, data, "message")
		assert.NotContains(t, data, "visit_id")
		assert.NotContains(t, data, "daily_checkout_available")
	})
	t.Run("returns 404 when no active session", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		_, _, personID := k.staff(t, "NoSession", "Staff")
		tag := k.card(t, "STAFFNS", personID)

		rr := k.call(t, "POST", "/checkin", map[string]any{"student_rfid": tag, "action": "checkin"}, k.device(t, "staff-no-session"))

		testutil.AssertNotFound(t, rr)
		assert.Contains(t, rr.Body.String(), "no active session - please start an activity first")
	})
	t.Run("idempotent duplicate supervisor scan", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		device, deviceID := k.deviceRow(t, "staff-dup")
		_, staffID, personID := k.staff(t, "Duplicate", "Supervisor")
		tag := k.card(t, "STAFFDUP", personID)
		room := testpkg.CreateTestRoom(t, k.db, "Dup Supervisor Room")
		activity := testpkg.CreateTestActivityGroup(t, k.db, "Dup Supervisor Activity")
		session := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, room.ID)
		testpkg.LinkDeviceToActiveGroup(t, k.db, session.ID, deviceID)
		testpkg.CreateTestGroupSupervisor(t, k.db, staffID, session.ID, "supervisor")

		rr := k.call(t, "POST", "/checkin", map[string]any{"student_rfid": tag, "action": "checkin"}, device)

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "supervisor_authenticated", data["action"])
		assert.Equal(t, room.Name, data["room_name"])
		assert.Equal(t, "Supervisor authenticated for Dup Supervisor Activity", data["message"])
		assert.Equal(t, "Duplicate Supervisor", data["student_name"])
		assert.Equal(t, "success", data["status"])
		assert.Contains(t, data, "processed_at")
	})
}

func TestDeviceCheckin_PersonNeitherStudentNorStaff(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	person := testpkg.CreateTestPerson(t, k.db, "Bare", "Person")
	tag := k.card(t, "BARE", person.ID)

	rr := k.call(t, "POST", "/checkin", map[string]any{"student_rfid": tag, "action": "checkin"}, k.device(t, "bare-person"))

	testutil.AssertNotFound(t, rr)
	assert.Contains(t, rr.Body.String(), "RFID tag not assigned to student or staff")
}

// ---------------------------------------------------------------------------
// Active student counts and session heartbeat
// ---------------------------------------------------------------------------

func TestDeviceCheckin_ActiveStudentsScopedToDeviceSession(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "shared-room-count")
	staff, _, _ := k.staff(t, "Shared", "Room")
	tag, scannedID, _ := k.studentCard(t, "Scanned", "Student", "3a")
	peer := testpkg.CreateTestStudent(t, k.db, "Session", "Peer", "3a")
	other := testpkg.CreateTestStudent(t, k.db, "Other", "Group", "3b")
	room := testpkg.CreateTestRoom(t, k.db, "Shared Count Room")
	deviceActivity := testpkg.CreateTestActivityGroup(t, k.db, "Device Session Activity")
	otherActivity := testpkg.CreateTestActivityGroup(t, k.db, "Other Session Activity")
	deviceGroup := testpkg.CreateTestActiveGroup(t, k.db, deviceActivity.ID, room.ID)
	otherGroup := testpkg.CreateTestActiveGroup(t, k.db, otherActivity.ID, room.ID)
	testpkg.LinkDeviceToActiveGroup(t, k.db, deviceGroup.ID, deviceID)
	testpkg.CreateTestVisit(t, k.db, scannedID, deviceGroup.ID, time.Now().Add(-10*time.Minute), nil)
	testpkg.CreateTestVisit(t, k.db, peer.ID, deviceGroup.ID, time.Now().Add(-8*time.Minute), nil)
	testpkg.CreateTestVisit(t, k.db, other.ID, otherGroup.ID, time.Now().Add(-6*time.Minute), nil)
	before := testpkg.ActiveGroupLastActivity(t, k.db, deviceGroup.ID)
	time.Sleep(10 * time.Millisecond)

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, room.ID), device, staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, float64(1), data["active_students"], "only the remaining students of the device session count")
	assert.False(t, testpkg.ActiveGroupLastActivity(t, k.db, deviceGroup.ID).Before(before), "the scan refreshes the session heartbeat")
}

func TestDeviceCheckin_ActiveStudentsCountsRoomWithoutDeviceLink(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Multi", "Staff")
	tag1, _, _ := k.studentCard(t, "First", "Counter", "1a")
	tag2, _, _ := k.studentCard(t, "Second", "Counter", "1b")
	roomID, _, _ := k.roomWithSession(t, "Multi Count Room")
	device := k.device(t, "multi-count")

	testutil.AssertSuccessResponse(t, k.call(t, "POST", "/checkin", checkinBody(tag1, roomID), device, staff), 200)
	rr := k.call(t, "POST", "/checkin", checkinBody(tag2, roomID), device, staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, float64(2), responseData(t, rr.Body.Bytes())["active_students"])
}

// ---------------------------------------------------------------------------
// Capacity
// ---------------------------------------------------------------------------

func TestDeviceCheckin_RoomCapacityExceeded(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Cap", "Staff")
	room := testpkg.CreateTestRoomWithCapacity(t, k.db, "Tiny Room", 1)
	activity := testpkg.CreateTestActivityGroup(t, k.db, "Capacity Activity")
	session := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, room.ID)
	existing := testpkg.CreateTestStudent(t, k.db, "Existing", "Student", "1a")
	testpkg.CreateTestVisit(t, k.db, existing.ID, session.ID, time.Now().Add(-10*time.Minute), nil)
	tag, _, _ := k.studentCard(t, "Over", "Capacity", "1b")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, room.ID), k.device(t, "capacity"), staff)

	require.Equal(t, 409, rr.Code, rr.Body.String())
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "ROOM_CAPACITY_EXCEEDED", response["code"])
	assert.Equal(t, "Room capacity exceeded", response["message"])
	details, ok := response["details"].(map[string]any)
	require.True(t, ok, "room details are disclosed by default")
	assert.EqualValues(t, 1, details["current_occupancy"])
	assert.EqualValues(t, 1, details["max_capacity"])
}

func TestDeviceCheckin_RoomCapacityExceededRollsBackSourceCheckout(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Capacity", "Transfer")
	sourceRoom := testpkg.CreateTestRoom(t, k.db, "Capacity Transfer Source")
	targetRoom := testpkg.CreateTestRoomWithCapacity(t, k.db, "Capacity Transfer Target", 1)
	activity := testpkg.CreateTestActivityGroup(t, k.db, "capacity-transfer-rollback")
	sourceGroup := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, sourceRoom.ID)
	targetGroup := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, targetRoom.ID)
	tag, movingID, _ := k.studentCard(t, "Moving", "Student", "1a")
	present := testpkg.CreateTestStudent(t, k.db, "Present", "Student", "1a")
	sourceVisit := testpkg.CreateTestVisit(t, k.db, movingID, sourceGroup.ID, time.Now().Add(-time.Hour), nil)
	testpkg.CreateTestVisit(t, k.db, present.ID, targetGroup.ID, time.Now().Add(-time.Hour), nil)

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, targetRoom.ID), k.device(t, "capacity-rollback"), staff)

	require.Equal(t, 409, rr.Code)
	persisted, err := k.presence(t).FindVisit(testpkg.Ctx(t), sourceVisit.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	assert.Nil(t, persisted.ExitTime, "the refused transfer rolled the source checkout back")
	assert.Equal(t, sourceGroup.ID, persisted.ActiveGroupID)
}

func TestDeviceCheckin_ActivityCapacityExceeded(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "ActCap", "Staff")
	room := testpkg.CreateTestRoom(t, k.db, "Activity Cap Room")
	activity := testpkg.CreateTestActivityGroupWithLimit(t, k.db, "Tiny Activity", 1)
	session := testpkg.CreateTestActiveGroup(t, k.db, activity.ID, room.ID)
	existing := testpkg.CreateTestStudent(t, k.db, "Existing", "ActCap", "1a")
	testpkg.CreateTestVisit(t, k.db, existing.ID, session.ID, time.Now().Add(-10*time.Minute), nil)
	tag, _, _ := k.studentCard(t, "Over", "ActCap", "1b")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, room.ID), k.device(t, "act-cap"), staff)

	require.Equal(t, 409, rr.Code, rr.Body.String())
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "ACTIVITY_CAPACITY_EXCEEDED", response["code"])
	assert.NotContains(t, response, "details", "activity details stay hidden by default")
}

// ---------------------------------------------------------------------------
// Special rooms: Schulhof and WC provision their own session
// ---------------------------------------------------------------------------

func TestDeviceCheckin_SchulhofProvisionsSession(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Schulhof", "Staff")
	tag1, _, _ := k.studentCard(t, "First", "Schulhof", "1a")
	tag2, _, _ := k.studentCard(t, "Second", "Schulhof", "1b")
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", true)
	device := k.device(t, "schulhof-auto")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag1, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "Schulhof", data["room_name"])

	// The second scan reuses the provisioned session instead of failing.
	rr = k.callIsolated(t, "POST", "/checkin", checkinBody(tag2, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "Schulhof", responseData(t, rr.Body.Bytes())["room_name"])

	session := testpkg.LatestActiveGroupInRoom(t, k.db, yard.ID)
	assert.Nil(t, session.DeviceID, "shared rooms are not device sessions")
}

func TestDeviceCheckin_SchulhofWithoutStaffContext(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "schulhof-no-staff")
	_, staffID, _ := k.staff(t, "SetupStaff", "SchulhofNoStaff")
	tag, studentID, _ := k.studentCard(t, "SchulhofNoStaff", "Student", "1b")
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", true)
	// A child reaches the yard only after a morning check-in with staff.
	testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-2*time.Hour), nil)

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, yard.ID), device)

	assert.Equal(t, 200, rr.Code, rr.Body.String())
}

func TestDeviceCheckin_WCProvisionsSessionAndChecksOut(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "WCOut", "Staff")
	tag, _, _ := k.studentCard(t, "WCOut", "Student", "2a")
	wc := testpkg.CreateTestRoomNamed(t, k.db, "WC")
	device := k.device(t, "wc-checkout")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, wc.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "WC", data["room_name"])
	assert.Nil(t, testpkg.LatestActiveGroupInRoom(t, k.db, wc.ID).DeviceID)

	rr = k.callIsolated(t, "POST", "/checkin", checkoutBody(tag), device)
	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "checked_out", responseData(t, rr.Body.Bytes())["action"])
}

func TestDeviceCheckin_WCWithoutStaffContext(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "wc-no-staff")
	_, staffID, _ := k.staff(t, "SetupStaff", "WCNoStaff")
	tag, studentID, _ := k.studentCard(t, "WCNoStaff", "Student", "1a")
	wc := testpkg.CreateTestRoomNamed(t, k.db, "WC")
	testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-2*time.Hour), nil)

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, wc.ID), device)

	assert.Equal(t, 200, rr.Code, rr.Body.String())
}

// A WC session must never hijack the device's own room session (regression
// for the commit that once linked provisioned groups to the device).
func TestDeviceCheckin_WCGroupDoesNotHijackDeviceSession(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	device, deviceID := k.deviceRow(t, "wc-hijack-regression")
	staff, _, _ := k.staff(t, "Regression", "Staff")
	tag, studentID, _ := k.studentCard(t, "Regression", "Student", "2a")
	_, _, sessionID := k.roomWithSession(t, "Session Room Regression")
	testpkg.LinkDeviceToActiveGroup(t, k.db, sessionID, deviceID)
	testpkg.CreateTestVisit(t, k.db, studentID, sessionID, time.Now().Add(-5*time.Minute), nil)
	wc := testpkg.CreateTestRoomNamed(t, k.db, "WC")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, wc.ID), device, staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Nil(t, testpkg.LatestActiveGroupInRoom(t, k.db, wc.ID).DeviceID)

	// The device still reports its own session, not the WC one.
	ping := k.callIsolated(t, "POST", "/ping", nil, device)
	testutil.AssertSuccessResponse(t, ping, 200)
	assert.Equal(t, true, responseData(t, ping.Body.Bytes())["session_active"])
}

func TestDeviceCheckin_ToiletteAliasUsesWCProvisioning(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Toilette", "Staff")
	tag, _, _ := k.studentCard(t, "Toilette", "Student", "1a")
	toilet := testpkg.CreateTestRoomNamed(t, k.db, "Toilette")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, toilet.ID), k.device(t, "toilette-auto"), staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_in", data["action"])
	assert.Equal(t, "Toilette", data["room_name"])
	assert.Nil(t, testpkg.LatestActiveGroupInRoom(t, k.db, toilet.ID).DeviceID)
	assert.Equal(t, 1, testpkg.CountRoomsNamed(t, k.db, "WC", "Toilette"), "the alias is reused, no duplicate WC room")
}

// ---------------------------------------------------------------------------
// Daily checkout from the yard (#2377) and from ordinary rooms
// ---------------------------------------------------------------------------

func TestDeviceCheckout_SchulhofOffersNachHauseWithoutAutoSendingHome(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "SchulhofHome", "Staff")
	tag, studentID, _ := k.studentCard(t, "SchulhofHome", "Student", "1a")
	// The child's group HAS its own room, which is what made the old room
	// gate reject every Schulhof checkout.
	groupRoom := testpkg.CreateTestRoom(t, k.db, "Hausaufgaben 1/2")
	group := testpkg.CreateTestEducationGroup(t, k.db, "Hausaufgaben")
	testpkg.SetEducationGroupRoom(t, k.db, group.ID, &groupRoom.ID)
	testpkg.SetStudentGroup(t, k.db, studentID, &group.ID)
	yard := testpkg.CreateTestSystemRoom(t, k.db, "Schulhof", true)
	device := k.device(t, "schulhof-home")

	rr := k.callIsolated(t, "POST", "/checkin", checkinBody(tag, yard.ID), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)
	require.Equal(t, "Schulhof", responseData(t, rr.Body.Bytes())["room_name"])

	rr = k.callIsolated(t, "POST", "/checkin", checkoutBody(tag), device)
	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, true, data["daily_checkout_available"], `the yard kiosk must offer "nach Hause"`)
	assert.Equal(t, "checked_out", data["action"], "the yard must not auto-send the child home")
}

func TestDeviceCheckout_OrdinaryRoomOffersNachHauseWithoutAutoSendingHome(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "Ordinary", "Staff")
	tag, studentID, _ := k.studentCard(t, "Ordinary", "Student", "1a")
	groupRoom := testpkg.CreateTestRoom(t, k.db, "Hausaufgaben 3/4")
	group := testpkg.CreateTestEducationGroup(t, k.db, "Hausaufgaben34")
	testpkg.SetEducationGroupRoom(t, k.db, group.ID, &groupRoom.ID)
	testpkg.SetStudentGroup(t, k.db, studentID, &group.ID)
	otherRoom, _, _ := k.roomWithSession(t, "Musikraum")
	device := k.device(t, "ordinary-room")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, otherRoom), device, staff)
	testutil.AssertSuccessResponse(t, rr, 200)

	rr = k.call(t, "POST", "/checkin", checkoutBody(tag), device)
	testutil.AssertSuccessResponse(t, rr, 200)
	data := responseData(t, rr.Body.Bytes())
	assert.Equal(t, "checked_out", data["action"])
	assert.Equal(t, true, data["daily_checkout_available"], "a new tenant offers nach Hause from an ordinary room")
}

// ---------------------------------------------------------------------------
// Pickup time in the scan response and the pickup query
// ---------------------------------------------------------------------------

// pickupClock pins the scan to a Monday, so a weekday pickup plan applies.
var pickupClock = func() time.Time { return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC) }

func TestDeviceCheckin_ResponseIncludesPickupTime(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t, pickupClock)
	staff, staffID, _ := k.staff(t, "Pickup", "Staff")
	tag, studentID, _ := k.studentCard(t, "Pickup", "Student", "2a")
	roomID, _, _ := k.roomWithSession(t, "Pickup Room")
	testpkg.CreateTestPickupSchedule(t, k.db, studentID, 1, staffID, "15:30")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomID), k.device(t, "pickup-time"), staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.Equal(t, "15:30", responseData(t, rr.Body.Bytes())["pickup_time"])
}

func TestDeviceCheckin_ResponseOmitsPickupTimeWithoutSchedule(t *testing.T) {
	t.Parallel()
	k := setupCheckinRoute(t)
	staff, _, _ := k.staff(t, "NoPickup", "Staff")
	tag, _, _ := k.studentCard(t, "NoPickup", "Student", "3b")
	roomID, _, _ := k.roomWithSession(t, "NoPickup Room")

	rr := k.call(t, "POST", "/checkin", checkinBody(tag, roomID), k.device(t, "no-pickup"), staff)

	testutil.AssertSuccessResponse(t, rr, 200)
	assert.NotContains(t, responseData(t, rr.Body.Bytes()), "pickup_time")
}

func TestDevicePickupQuery(t *testing.T) {
	t.Parallel()
	monday := testpkg.NewCalendarDate(2026, 8, 24)

	t.Run("returns pickup info without creating a visit", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t, pickupClock)
		staff, staffID, _ := k.staff(t, "PickupQuery", "Staff")
		tag, studentID, _ := k.studentCard(t, "PickupQuery", "Student", "2a")
		testpkg.CreateTestPickupSchedule(t, k.db, studentID, 1, staffID, "15:30")
		testpkg.CreateTestPickupNote(t, k.db, studentID, monday, staffID, "Mama holt heute frueher ab")

		rr := k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, k.device(t, "pickup-query"), staff)

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "pickup_info", data["action"])
		assert.Equal(t, "15:30", data["pickup_time"])
		assert.Equal(t, "Mama holt heute frueher ab", data["pickup_note"])
		assert.Empty(t, k.openVisits(t, studentID), "a pickup query must not create visits")
	})
	t.Run("omits time and note without a plan", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		tag, _, _ := k.studentCard(t, "EmptyPickup", "Student", "3b")

		rr := k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, k.device(t, "pickup-empty"))

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "pickup_info", data["action"])
		assert.NotContains(t, data, "pickup_time")
		assert.NotContains(t, data, "pickup_note")
	})
	t.Run("prefers day notes over the recurring note", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t, pickupClock)
		_, staffID, _ := k.staff(t, "PickupNotes", "Staff")
		tag, studentID, _ := k.studentCard(t, "PickupNotes", "Student", "2b")
		testpkg.CreateTestPickupSchedule(t, k.db, studentID, 1, staffID, "15:30")
		testpkg.CreateTestPickupNote(t, k.db, studentID, monday, staffID, "Heute holt Oma ab")
		testpkg.CreateTestPickupNote(t, k.db, studentID, monday, staffID, "Bitte am Seiteneingang warten")

		rr := k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, k.device(t, "pickup-notes"))

		testutil.AssertSuccessResponse(t, rr, 200)
		assert.Equal(t, "Heute holt Oma ab\nBitte am Seiteneingang warten", responseData(t, rr.Body.Bytes())["pickup_note"])
	})
	t.Run("an exception time with a blank reason keeps the recurring note", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t, pickupClock)
		_, staffID, _ := k.staff(t, "PickupException", "Staff")
		tag, studentID, _ := k.studentCard(t, "PickupException", "Student", "2c")
		testpkg.CreateTestPickupScheduleWithNote(t, k.db, studentID, 1, staffID, "15:30", "Bitte am Seiteneingang klingeln")
		testpkg.CreateTestPickupException(t, k.db, studentID, monday, staffID, "13:00", "   ")

		rr := k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, k.device(t, "pickup-exception"))

		testutil.AssertSuccessResponse(t, rr, 200)
		data := responseData(t, rr.Body.Bytes())
		assert.Equal(t, "13:00", data["pickup_time"])
		assert.Equal(t, "Bitte am Seiteneingang klingeln", data["pickup_note"])
	})
	t.Run("rejects a staff card", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		staff, _, personID := k.staff(t, "PickupStaff", "Only")
		tag := k.card(t, "STAFFPICKUP", personID)

		rr := k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, k.device(t, "pickup-staff"), staff)

		testutil.AssertBadRequest(t, rr)
		assert.Contains(t, rr.Body.String(), "student RFID tag required for pickup query")
	})
	t.Run("a card of neither student nor staff", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		person := testpkg.CreateTestPerson(t, k.db, "NeitherPickup", "Person")
		tag := k.card(t, "NEITHER", person.ID)

		testutil.AssertNotFound(t, k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": tag}, k.device(t, "pickup-neither")))
	})
	t.Run("request contract", func(t *testing.T) {
		t.Parallel()
		k := setupCheckinRoute(t)
		device := k.device(t, "pickup-contract")
		testutil.AssertUnauthorized(t, k.call(t, "POST", "/pickup-query", nil))
		testutil.AssertBadRequest(t, k.call(t, "POST", "/pickup-query", map[string]any{}, device))
		testutil.AssertNotFound(t, k.call(t, "POST", "/pickup-query", map[string]any{"student_rfid": "NONEXISTENT_RFID_TAG_XYZ"}, device))
	})
}
