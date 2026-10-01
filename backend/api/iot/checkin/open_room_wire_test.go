package checkin_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	checkinAPI "github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// The destination booking (#3067) pins the status codes and bodies PyrePortal
// maps. The workflow is a fake, so the route's binding and renderer are the
// only things under test; the booking itself is covered by the device-scan
// application tests and the open-room move tests.

type fakeOpenRoomBooking struct {
	noDev    bool
	result   *devicescan.OpenRoomResult
	err      error
	commands []devicescan.OpenRoomCommand
}

func (f *fakeOpenRoomBooking) Device(context.Context) (devicescan.Device, error) {
	if f.noDev {
		return devicescan.Device{}, devicescan.ErrDeviceUnauthorized
	}
	return devicescan.Device{DeviceID: "dev-001"}, nil
}

func (f *fakeOpenRoomBooking) BookOpenRoom(_ context.Context, command devicescan.OpenRoomCommand) (*devicescan.OpenRoomResult, error) {
	f.commands = append(f.commands, command)
	return f.result, f.err
}

func bookWith(t *testing.T, booking *fakeOpenRoomBooking, body any) *httptest.ResponseRecorder {
	t.Helper()
	resource := checkinAPI.NewOpenRoomResource(booking, testRuntime(), nil)
	req := testutil.NewAuthenticatedRequest(t, "POST", "/move-to-room", body)
	return testutil.ExecuteRequest(resource.Router(), req)
}

func TestOpenRoomWire_BookedStay(t *testing.T) {
	t.Parallel()
	booking := &fakeOpenRoomBooking{result: &devicescan.OpenRoomResult{
		StudentID: 5, StudentName: "Max Muster", RoomID: 77, RoomName: "Turnhalle", RoomSessionID: 250,
		Moved: true, Action: devicescan.ScanActionOpenRoomStay, Message: "Max ist jetzt in Turnhalle.",
	}}

	rr := bookWith(t, booking, map[string]any{"student_rfid": "TAG", "room_id": 77})

	require.Equal(t, 200, rr.Code, rr.Body.String())
	assert.Equal(t, []devicescan.OpenRoomCommand{{RFIDTag: "TAG", RoomID: 77}}, booking.commands)
	response := testutil.ParseJSONResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response["status"])
	assert.Equal(t, "Max ist jetzt in Turnhalle.", response["message"])
	data, ok := response["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(5), data["student_id"])
	assert.Equal(t, "Max Muster", data["student_name"])
	assert.Equal(t, "open_room_stay", data["action"])
	assert.Equal(t, float64(77), data["room_id"])
	assert.Equal(t, "Turnhalle", data["room_name"])
	assert.Equal(t, float64(250), data["active_group_id"])
	assert.Equal(t, true, data["moved"])
	assert.Equal(t, "Max ist jetzt in Turnhalle.", data["message"])
	assert.NotEmpty(t, data["processed_at"])
}

func TestOpenRoomWire_RepeatedBookingIsStillSuccess(t *testing.T) {
	t.Parallel()
	booking := &fakeOpenRoomBooking{result: &devicescan.OpenRoomResult{StudentID: 5, RoomID: 77, RoomSessionID: 250, Action: devicescan.ScanActionOpenRoomStay}}

	rr := bookWith(t, booking, map[string]any{"student_rfid": "TAG", "room_id": 77})

	require.Equal(t, 200, rr.Code)
	assert.Contains(t, rr.Body.String(), `"moved":false`)
}

func TestOpenRoomWire_InvalidRequestsNeverReachTheWorkflow(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]map[string]any{
		"missing card":  {"room_id": 77},
		"missing room":  {"student_rfid": "TAG"},
		"negative room": {"student_rfid": "TAG", "room_id": -1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			booking := &fakeOpenRoomBooking{}
			rr := bookWith(t, booking, body)
			assert.Equal(t, 400, rr.Code, rr.Body.String())
			assert.Empty(t, booking.commands)
		})
	}
}

func TestOpenRoomWire_RequiresADevice(t *testing.T) {
	t.Parallel()
	booking := &fakeOpenRoomBooking{noDev: true}

	rr := bookWith(t, booking, map[string]any{"student_rfid": "TAG", "room_id": 77})

	assert.Equal(t, 401, rr.Code)
	assert.JSONEq(t, `{"status":"error","error":"device API key is required"}`, rr.Body.String())
	assert.Empty(t, booking.commands)
}

func TestOpenRoomWire_ClassifiedRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{"unknown room", devicescan.NotFoundWithCode(devicescan.MessageOpenRoomNotFound, devicescan.CodeOpenRoomNotFound), 404, `{"status":"error","error":"room not found","code":"room_not_found"}`},
		{"room not released", &devicescan.Failure{Kind: devicescan.FailureConflict, Message: devicescan.MessageOpenRoomNotReleased, Code: devicescan.CodeOpenRoomNotReleased}, 409, `{"status":"error","error":"room is not released as an open room","code":"room_not_released"}`},
		{"child not checked in", &devicescan.Failure{Kind: devicescan.FailureConflict, Message: devicescan.MessageStudentNotPresent, Code: devicescan.CodeStudentNotPresent}, 409, `{"status":"error","error":"student is not checked in","code":"student_not_present"}`},
		{"binary mode", &devicescan.Failure{Kind: devicescan.FailureConflict, Message: devicescan.MessageOpenRoomBinaryMode, Code: devicescan.CodeOpenRoomBinaryMode}, 409, `{"status":"error","error":"open rooms need detailed presence mode","code":"open_room_binary_mode"}`},
		{"unknown card", devicescan.NotFoundWithCode(devicescan.MessageRFIDTagNotFound, devicescan.CodeRFIDTagNotFound), 404, `{"status":"error","error":"RFID tag not found","code":"rfid_tag_not_found"}`},
		{"full room", &devicescan.RoomCapacityExceededError{RoomID: 77, RoomName: "Turnhalle", CurrentOccupancy: 20, MaxCapacity: 20}, 409, `{"status":"error","message":"Room capacity exceeded","code":"ROOM_CAPACITY_EXCEEDED"}`},
		{"active elsewhere", &devicescan.StudentAlreadyActiveError{StudentID: 5}, 409, `{"status":"error","message":"student already has an active visit","code":"STUDENT_ALREADY_ACTIVE","details":{"student_id":5}}`},
		{"server fault", devicescan.Internal(devicescan.MessageCreateVisitFailed, nil), 500, `{"status":"error","error":"failed to create visit record"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := bookWith(t, &fakeOpenRoomBooking{err: tc.err}, map[string]any{"student_rfid": "TAG", "room_id": 77})
			assert.Equal(t, tc.status, rr.Code)
			assert.JSONEq(t, tc.body, rr.Body.String())
		})
	}
}
