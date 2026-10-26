package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

// deviceSessionID is the device's running session in another room than the
// scanned one: only the device-session lookup (POST /session/activity's
// target) finds it.
const deviceSessionID int64 = 260

// A successful scan keeps the device session alive by itself, so the kiosk
// needs no POST /session/activity after it (#5004). Every scan path either
// refreshes the session the activity call refreshes or is named as exempt.
func TestScan_KeepsDeviceSessionAlive(t *testing.T) {
	t.Parallel()
	openVisitIn := func(sessionID, roomID int64, roomName string) *ports.CurrentVisit {
		return &ports.CurrentVisit{ID: 55, EntryTime: fixedNow.Add(-time.Hour), Session: &ports.SessionRef{
			ID: sessionID, RoomID: roomID, StartTime: fixedNow.Add(-time.Hour), Room: &ports.Room{ID: roomID, Name: roomName},
		}}
	}
	for _, tc := range []struct {
		name    string
		arrange func(*harness)
		command devicescan.ScanCommand
		action  string
	}{
		{
			name:    "detailed check-in into a room without the device session",
			command: scanCommand(ptr(testRoomID)),
			action:  devicescan.ScanActionCheckedIn,
		},
		{
			name:    "detailed room change into a room without the device session",
			arrange: func(h *harness) { h.visits.current = openVisitIn(150, 7, "Musikraum") },
			command: scanCommand(ptr(testRoomID)),
			action:  devicescan.ScanActionTransferred,
		},
		{
			name:    "detailed same-room checkout",
			arrange: func(h *harness) { h.visits.current = openVisitIn(201, testRoomID, "Klassenraum 1a") },
			command: scanCommand(ptr(testRoomID)),
			action:  devicescan.ScanActionCheckedOut,
		},
		{
			name:    "detailed checkout without a room",
			arrange: func(h *harness) { h.visits.current = openVisitIn(201, testRoomID, "Klassenraum 1a") },
			command: scanCommand(nil),
			action:  devicescan.ScanActionCheckedOut,
		},
		{
			name: "binary attendance toggle",
			arrange: func(h *harness) {
				h.settings.mode = presenceModeBinary
				h.attendance.toggleAction = devicescan.ScanActionCheckedIn
			},
			command: scanCommand(ptr(testRoomID)),
			action:  devicescan.ScanActionCheckedIn,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.sessions.current = &devicescan.Session{ID: deviceSessionID}
			if tc.arrange != nil {
				tc.arrange(h)
			}

			result, err := h.service().Scan(context.Background(), tc.command)

			require.NoError(t, err)
			assert.Equal(t, tc.action, result.Action)
			assert.Equal(t, []int64{deviceSessionID}, h.sessions.touched, "the device session gets exactly one heartbeat")
		})
	}
}

// The device's session in the scanned room is the one the activity call
// would refresh; the scan refreshes it once and looks nothing up again.
func TestScan_DeviceSessionInRoomGetsOneHeartbeat(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.sessions.deviceSession = &devicescan.Session{ID: 250}
	h.sessions.current = &devicescan.Session{ID: 250}

	_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

	require.NoError(t, err)
	assert.Equal(t, []int64{250}, h.sessions.touched)
}

// Exempt paths: a device without a running session has nothing to keep
// alive, a supervisor scan never needed the activity call, and a refused
// scan leaves the session as it was.
func TestScan_HeartbeatExemptions(t *testing.T) {
	t.Parallel()
	t.Run("a device without a running session scans as before", func(t *testing.T) {
		t.Parallel()
		for _, mode := range []string{presenceModeDetailed, presenceModeBinary} {
			h := newHarness(t)
			h.settings.mode = mode
			h.attendance.toggleAction = devicescan.ScanActionCheckedIn

			result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

			require.NoError(t, err, mode)
			assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action, mode)
			assert.Empty(t, h.sessions.touched, mode)
		}
	})
	t.Run("a failed device-session lookup only logs", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.sessions.currentErr = errBoom

		result, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

		require.NoError(t, err)
		assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action)
		assert.Empty(t, h.sessions.touched)
	})
	t.Run("a failed heartbeat keeps the scan", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.settings.mode = presenceModeBinary
		h.attendance.toggleAction = devicescan.ScanActionCheckedOut
		h.sessions.current = &devicescan.Session{ID: deviceSessionID}
		h.sessions.touchErr = errBoom

		result, err := h.service().Scan(context.Background(), scanCommand(nil))

		require.NoError(t, err)
		assert.Equal(t, devicescan.ScanActionCheckedOut, result.Action)
		assert.Equal(t, []int64{deviceSessionID}, h.sessions.touched, "the heartbeat was attempted")
	})
	t.Run("a supervisor scan", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		delete(h.people.students, testPersonID)
		h.people.staff[testPersonID] = &ports.StaffMember{ID: testStaffID}
		h.sessions.current = &devicescan.Session{ID: deviceSessionID}

		result, err := h.service().Scan(context.Background(), scanCommand(nil))

		require.NoError(t, err)
		assert.Equal(t, devicescan.ScanOutcomeSupervisor, result.Outcome)
		assert.Empty(t, h.sessions.touched)
	})
	t.Run("a refused scan", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.sessions.open = nil
		h.sessions.current = &devicescan.Session{ID: deviceSessionID}

		_, err := h.service().Scan(context.Background(), scanCommand(ptr(testRoomID)))

		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessageNoGroupsInRoom)
		assert.Empty(t, h.sessions.touched)
	})
}
