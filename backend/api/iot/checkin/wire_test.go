package checkin_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// The wire goldens pin the exact status code and body bytes PyrePortal
// receives for every classified outcome. They drive the real router with a
// fake workflow, so the renderer is the only thing under test; the
// go-chi/render pipeline emits compact JSON with a trailing newline.

type fakeScanner struct {
	device  devicescan.Device
	noDev   bool
	scanErr error
	scan    *devicescan.ScanResult
}

func (f *fakeScanner) Device(context.Context) (devicescan.Device, error) {
	if f.noDev {
		return devicescan.Device{}, devicescan.ErrDeviceUnauthorized
	}
	return f.device, nil
}
func (f *fakeScanner) Scan(context.Context, devicescan.ScanCommand) (*devicescan.ScanResult, error) {
	return f.scan, f.scanErr
}
func (f *fakeScanner) PickupInfo(context.Context, string) (*devicescan.PickupInfo, error) {
	return nil, f.scanErr
}
func (f *fakeScanner) Ping(context.Context) (*devicescan.DevicePing, error) { return nil, f.scanErr }
func (f *fakeScanner) Status(context.Context) (*devicescan.DeviceStatus, error) {
	return nil, f.scanErr
}

func scanWith(t *testing.T, scanner *fakeScanner) *httptest.ResponseRecorder {
	t.Helper()
	resource := checkinAPI.NewResource(scanner, testRuntime(), nil)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", map[string]any{"student_rfid": "TAG", "action": "checkin", "room_id": 42})
	return testutil.ExecuteRequest(resource.Router(), req)
}

func TestWireFormat_RoomCapacityExceeded(t *testing.T) {
	t.Parallel()
	rr := scanWith(t, &fakeScanner{scanErr: &devicescan.RoomCapacityExceededError{RoomID: 42, RoomName: "Room A", CurrentOccupancy: 15, MaxCapacity: 10, Details: true}})
	assert.Equal(t, 409, rr.Code)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"Room capacity exceeded\",\"code\":\"ROOM_CAPACITY_EXCEEDED\",\"details\":{\"room_id\":42,\"room_name\":\"Room A\",\"current_occupancy\":15,\"max_capacity\":10}}\n",
		rr.Body.String(),
	)
}

func TestWireFormat_RoomCapacityExceeded_NoDetails(t *testing.T) {
	t.Parallel()
	rr := scanWith(t, &fakeScanner{scanErr: &devicescan.RoomCapacityExceededError{RoomID: 42, RoomName: "Room A", CurrentOccupancy: 15, MaxCapacity: 10}})
	assert.Equal(t, 409, rr.Code)
	assert.Equal(t, "{\"status\":\"error\",\"message\":\"Room capacity exceeded\",\"code\":\"ROOM_CAPACITY_EXCEEDED\"}\n", rr.Body.String())
}

func TestWireFormat_ActivityCapacityExceeded(t *testing.T) {
	t.Parallel()
	rr := scanWith(t, &fakeScanner{scanErr: &devicescan.ActivityCapacityExceededError{ActivityID: 7, ActivityName: "Bastelraum", CurrentOccupancy: 5, MaxCapacity: 4, Details: true}})
	assert.Equal(t, 409, rr.Code)
	assert.Equal(t,
		"{\"status\":\"error\",\"message\":\"Activity capacity exceeded\",\"code\":\"ACTIVITY_CAPACITY_EXCEEDED\",\"details\":{\"activity_id\":7,\"activity_name\":\"Bastelraum\",\"current_occupancy\":5,\"max_capacity\":4}}\n",
		rr.Body.String(),
	)
}

func TestWireFormat_ActivityCapacityExceeded_NoDetails(t *testing.T) {
	t.Parallel()
	rr := scanWith(t, &fakeScanner{scanErr: &devicescan.ActivityCapacityExceededError{ActivityID: 7, ActivityName: "Bastelraum", CurrentOccupancy: 5, MaxCapacity: 4}})
	assert.Equal(t, 409, rr.Code)
	assert.Equal(t, "{\"status\":\"error\",\"message\":\"Activity capacity exceeded\",\"code\":\"ACTIVITY_CAPACITY_EXCEEDED\"}\n", rr.Body.String())
}

func TestWireFormat_StudentAlreadyActive(t *testing.T) {
	t.Parallel()
	t.Run("degraded path carries only the student", func(t *testing.T) {
		t.Parallel()
		rr := scanWith(t, &fakeScanner{scanErr: &devicescan.StudentAlreadyActiveError{StudentID: 99}})
		assert.Equal(t, 409, rr.Code)
		assert.Equal(t, "{\"status\":\"error\",\"message\":\"student already has an active visit\",\"code\":\"STUDENT_ALREADY_ACTIVE\",\"details\":{\"student_id\":99}}\n", rr.Body.String())
	})
	t.Run("full path carries the existing stay", func(t *testing.T) {
		t.Parallel()
		entry := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
		roomID := int64(5)
		rr := scanWith(t, &fakeScanner{scanErr: &devicescan.StudentAlreadyActiveError{StudentID: 99, ExistingVisitID: 123, EntryTime: &entry, RoomID: &roomID, RoomName: "Room A"}})
		assert.Equal(t, 409, rr.Code)
		assert.Equal(t,
			"{\"status\":\"error\",\"message\":\"student already has an active visit\",\"code\":\"STUDENT_ALREADY_ACTIVE\",\"details\":{\"student_id\":99,\"existing_visit_id\":123,\"entry_time\":\"2026-07-12T10:30:00Z\",\"room_id\":5,\"room_name\":\"Room A\"}}\n",
			rr.Body.String(),
		)
	})
}

func TestWireFormat_ClassifiedFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"unauthorized", devicescan.ErrDeviceUnauthorized, 401, `{"status":"error","error":"device API key is required"}`},
		{"not found with code", devicescan.NotFoundWithCode(devicescan.MessageRFIDTagNotFound, devicescan.CodeRFIDTagNotFound), 404, `{"status":"error","error":"RFID tag not found","code":"rfid_tag_not_found"}`},
		{"not found", devicescan.NotFound(devicescan.MessageNoGroupsInRoom), 404, `{"status":"error","error":"no active groups in specified room"}`},
		{"invalid request", devicescan.InvalidRequest(devicescan.MessageRoomIDRequired), 400, `{"status":"error","error":"room_id is required for check-in"}`},
		{"conflict", devicescan.Conflict("active: op: session conflict detected"), 409, `{"status":"error","error":"active: op: session conflict detected"}`},
		{"internal with hidden cause", devicescan.Internal(devicescan.MessageInternalServerError, errors.New("rfid lookup exploded")), 500, `{"status":"error","error":"Internal server error"}`},
		{"internal pinned text", devicescan.Internal(devicescan.MessageCreateVisitFailed, nil), 500, `{"status":"error","error":"failed to create visit record"}`},
		{"unknown error", errors.New("something unexpected"), 500, `{"status":"error","error":"something unexpected"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scanner := &fakeScanner{scanErr: tc.err}
			if errors.Is(tc.err, devicescan.ErrDeviceUnauthorized) {
				scanner.noDev = true
			}
			rr := scanWith(t, scanner)
			assert.Equal(t, tc.status, rr.Code)
			assert.JSONEq(t, tc.body, rr.Body.String())
			assert.NotContains(t, rr.Body.String(), "rfid lookup exploded")
		})
	}
}

func TestWireFormat_ScanOutcomes(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC)
	visitID := int64(77)
	count := 3
	pickup := "15:30"

	t.Run("visit", func(t *testing.T) {
		t.Parallel()
		rr := scanWith(t, &fakeScanner{scan: &devicescan.ScanResult{
			Outcome: devicescan.ScanOutcomeVisit, PersonID: 1, PersonName: "Max Muster", Action: "transferred", VisitID: &visitID,
			RoomName: "Room B", PreviousRoomName: "Room A", Message: "Gewechselt von Room A zu Room B!", DailyCheckoutAvailable: true,
			FeedbackEnabled: true, ActiveStudents: &count, PickupTime: &pickup, ProcessedAt: at,
		}})
		require.Equal(t, 200, rr.Code)
		assert.JSONEq(t, `{"status":"success","message":"Student transferred successfully","data":{
			"student_id":1,"student_name":"Max Muster","action":"transferred","visit_id":77,"room_name":"Room B",
			"processed_at":"2026-08-05T09:00:00Z","message":"Gewechselt von Room A zu Room B!","status":"success",
			"daily_checkout_available":true,"feedback_enabled":true,"previous_room":"Room A","active_students":3,"pickup_time":"15:30"}}`, rr.Body.String())
	})
	t.Run("visit without a visit id keeps the null key", func(t *testing.T) {
		t.Parallel()
		rr := scanWith(t, &fakeScanner{scan: &devicescan.ScanResult{Outcome: devicescan.ScanOutcomeVisit, PersonID: 1, PersonName: "Max Muster", Action: "no_action", Message: "Keine Aktion durchgeführt", ProcessedAt: at}})
		require.Equal(t, 200, rr.Code)
		assert.Contains(t, rr.Body.String(), `"visit_id":null`)
		assert.NotContains(t, rr.Body.String(), "previous_room")
		assert.NotContains(t, rr.Body.String(), "active_students")
	})
	t.Run("binary attendance", func(t *testing.T) {
		t.Parallel()
		rr := scanWith(t, &fakeScanner{scan: &devicescan.ScanResult{Outcome: devicescan.ScanOutcomeAttendance, PersonID: 1, PersonName: "Max Muster", Action: "checked_in", Message: "Willkommen, Max!", ProcessedAt: at}})
		require.Equal(t, 200, rr.Code)
		assert.JSONEq(t, `{"status":"success","message":"Student checked_in successfully","data":{
			"student_id":1,"student_name":"Max Muster","action":"checked_in","room_name":"","processed_at":"2026-08-05T09:00:00Z",
			"message":"Willkommen, Max!","status":"success","daily_checkout_available":false,"feedback_enabled":false}}`, rr.Body.String())
	})
	t.Run("supervisor", func(t *testing.T) {
		t.Parallel()
		rr := scanWith(t, &fakeScanner{scan: &devicescan.ScanResult{Outcome: devicescan.ScanOutcomeSupervisor, PersonID: 42, PersonName: "Staff Member", Action: "supervisor_authenticated", RoomName: "Kreativraum", Message: "Supervisor authenticated for Basteln", ProcessedAt: at}})
		require.Equal(t, 200, rr.Code)
		assert.JSONEq(t, `{"status":"success","message":"Supervisor authenticated","data":{
			"student_id":42,"student_name":"Staff Member","action":"supervisor_authenticated","room_name":"Kreativraum",
			"processed_at":"2026-08-05T09:00:00Z","message":"Supervisor authenticated for Basteln","status":"success"}}`, rr.Body.String())
	})
}
