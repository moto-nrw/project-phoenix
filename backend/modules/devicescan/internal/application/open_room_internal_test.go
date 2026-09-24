package application

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// fakeOpenRooms records every move the service asks for and answers with a
// fixed outcome or error.
type fakeOpenRooms struct {
	outcome devicescan.OpenRoomMoveOutcome
	err     error
	moves   []devicescan.OpenRoomMove
}

func (m *fakeOpenRooms) MoveToOpenRoom(_ context.Context, move devicescan.OpenRoomMove) (devicescan.OpenRoomMoveOutcome, error) {
	m.moves = append(m.moves, move)
	return m.outcome, m.err
}

const testOpenRoomID = int64(77)

// newOpenRoomHarness adds a released gym and a mover that books into it.
func newOpenRoomHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.rooms.rooms[testOpenRoomID] = facilities.Room{ID: testOpenRoomID, Name: "Turnhalle", IsOpenRoom: true}
	h.openRooms = &fakeOpenRooms{outcome: devicescan.OpenRoomMoveOutcome{RoomSessionID: 250, Moved: true}}
	return h
}

func bookOpenRoom(h *harness) (*devicescan.OpenRoomResult, error) {
	return h.service().BookOpenRoom(context.Background(), devicescan.OpenRoomCommand{RFIDTag: testTag, RoomID: testOpenRoomID})
}

func TestBookOpenRoom_BooksTheScannedChildIntoTheRoom(t *testing.T) {
	t.Parallel()
	h := newOpenRoomHarness(t)

	result, err := bookOpenRoom(h)

	require.NoError(t, err)
	assert.Equal(t, []devicescan.OpenRoomMove{{StudentID: testStudentID, RoomID: testOpenRoomID}}, h.openRooms.moves)
	assert.Equal(t, &devicescan.OpenRoomResult{
		StudentID:     testStudentID,
		StudentName:   "Max Muster",
		RoomID:        testOpenRoomID,
		RoomName:      "Turnhalle",
		RoomSessionID: 250,
		Moved:         true,
		Action:        devicescan.ScanActionOpenRoomStay,
		Message:       "Max ist jetzt in Turnhalle.",
	}, result)
	assert.Zero(t, h.unit.rollbacks)
	assert.Empty(t, h.visits.recorded, "the device-scan visit path is not used")
	assert.Empty(t, h.sessions.started, "no activity session is started")
}

func TestBookOpenRoom_RepeatedBookingReportsNoMove(t *testing.T) {
	t.Parallel()
	h := newOpenRoomHarness(t)
	h.openRooms.outcome = devicescan.OpenRoomMoveOutcome{RoomSessionID: 250, Moved: false}

	result, err := bookOpenRoom(h)

	require.NoError(t, err)
	assert.False(t, result.Moved)
	assert.Equal(t, int64(250), result.RoomSessionID)
}

func TestBookOpenRoom_RoomNameLookupFailureRollsBack(t *testing.T) {
	t.Parallel()
	h := newOpenRoomHarness(t)
	h.rooms.findErr = errBoom

	result, err := bookOpenRoom(h)

	requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
	require.ErrorIs(t, err, errBoom)
	assert.Nil(t, result)
	assert.Len(t, h.openRooms.moves, 1)
	assert.Equal(t, 1, h.unit.rollbacks)
}

func TestBookOpenRoom_Refusals(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("requires a device", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		h.principals.device = nil
		_, err := bookOpenRoom(h)
		require.ErrorIs(t, err, devicescan.ErrDeviceUnauthorized)
		assert.Empty(t, h.openRooms.moves)
	})
	t.Run("a missing room is invalid", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		_, err := h.service().BookOpenRoom(ctx, devicescan.OpenRoomCommand{RFIDTag: testTag})
		requireFailure(t, err, devicescan.FailureInvalidRequest, devicescan.MessageOpenRoomIDRequired)
		assert.Empty(t, h.openRooms.moves)
	})
	t.Run("an unknown card is not found and recorded", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		_, err := h.service().BookOpenRoom(ctx, devicescan.OpenRoomCommand{RFIDTag: "UNKNOWN", RoomID: testOpenRoomID})
		failure, ok := devicescan.IsFailure(err)
		require.True(t, ok, "got %v", err)
		assert.Equal(t, devicescan.FailureNotFound, failure.Kind)
		assert.Empty(t, h.openRooms.moves)
	})
	t.Run("a staff card is not a student", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		delete(h.people.students, testPersonID)
		_, err := bookOpenRoom(h)
		requireFailure(t, err, devicescan.FailureNotFound, devicescan.MessagePersonNotStudent)
		assert.Empty(t, h.openRooms.moves)
	})
	t.Run("binary presence mode has no rooms to stay in", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		h.settings.mode = presenceModeBinary
		_, err := bookOpenRoom(h)
		failure := classifiedFailure(t, err, devicescan.FailureConflict, devicescan.MessageOpenRoomBinaryMode)
		assert.Equal(t, devicescan.CodeOpenRoomBinaryMode, failure.Code)
		assert.Empty(t, h.openRooms.moves)
	})
	t.Run("a presence mode lookup failure is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		h.settings.modeErr = errBoom
		_, err := bookOpenRoom(h)
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageInternalServerError)
	})
	t.Run("a graph without the mover is a server fault", func(t *testing.T) {
		t.Parallel()
		h := newOpenRoomHarness(t)
		h.openRooms = nil
		_, err := bookOpenRoom(h)
		requireFailure(t, err, devicescan.FailureInternal, devicescan.MessageOpenRoomUnavailable)
	})
}

func TestBookOpenRoom_ClassifiesMoverFailuresAndRollsBack(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		err     error
		kind    devicescan.FailureKind
		message string
		code    string
	}{
		{"unknown room", fmt.Errorf("%w: room not found", devicescan.ErrOpenRoomNotFound), devicescan.FailureNotFound, devicescan.MessageOpenRoomNotFound, devicescan.CodeOpenRoomNotFound},
		{"room not released", devicescan.ErrOpenRoomNotReleased, devicescan.FailureConflict, devicescan.MessageOpenRoomNotReleased, devicescan.CodeOpenRoomNotReleased},
		{"child not checked in", devicescan.ErrOpenRoomStudentNotPresent, devicescan.FailureConflict, devicescan.MessageStudentNotPresent, devicescan.CodeStudentNotPresent},
		{"anything else", errBoom, devicescan.FailureInternal, devicescan.MessageCreateVisitFailed, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newOpenRoomHarness(t)
			h.openRooms.err = tc.err

			_, err := bookOpenRoom(h)

			failure := classifiedFailure(t, err, tc.kind, tc.message)
			assert.Equal(t, tc.code, failure.Code)
			assert.Equal(t, 1, h.unit.rollbacks, "a refused move rolls the request back")
		})
	}
}

func TestBookOpenRoom_FullRoomKeepsTheCapacityBody(t *testing.T) {
	t.Parallel()
	for _, disclosed := range []bool{true, false} {
		t.Run(fmt.Sprintf("details disclosed %v", disclosed), func(t *testing.T) {
			t.Parallel()
			h := newOpenRoomHarness(t)
			h.settings.roomDetails = disclosed
			h.openRooms.err = &devicescan.RoomCapacityExceededError{RoomID: testOpenRoomID, RoomName: "Turnhalle", CurrentOccupancy: 20, MaxCapacity: 20}

			_, err := bookOpenRoom(h)

			capacity, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err)
			require.True(t, ok, "got %v", err)
			assert.Equal(t, disclosed, capacity.Details)
			assert.Equal(t, 20, capacity.MaxCapacity)
			assert.Equal(t, 1, h.unit.rollbacks)
		})
	}
}

func TestBookOpenRoom_ChildActiveElsewhereStaysAStudentConflict(t *testing.T) {
	t.Parallel()
	h := newOpenRoomHarness(t)
	h.openRooms.err = &devicescan.StudentAlreadyActiveError{StudentID: testStudentID}

	_, err := bookOpenRoom(h)

	active, ok := errors.AsType[*devicescan.StudentAlreadyActiveError](err)
	require.True(t, ok, "got %v", err)
	assert.Equal(t, testStudentID, active.StudentID)
	assert.Equal(t, 1, h.unit.rollbacks)
}
