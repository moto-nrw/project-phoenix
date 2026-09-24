package compose_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

var (
	errTestRoomNotFound    = errors.New("room not found")
	errTestRoomNotReleased = errors.New("room is not released as an open room")
)

// The mover is a pure translation, so the fake ids below name no rows.
const (
	testChildID int64 = 5
	testRoomID  int64 = 77
)

type recordedOpenRoomMove struct{ studentID, roomID int64 }

// openRoomMoverWith builds the mover over a move that answers with a fixed
// report or error and records what it was asked.
func openRoomMoverWith(report devicescanCompose.OpenRoomMoveReport, err error) (devicescanCompose.OpenRoomMover, *[]recordedOpenRoomMove) {
	var moves []recordedOpenRoomMove
	mover := devicescanCompose.NewOpenRoomMover(func(_ context.Context, studentID, roomID int64) (devicescanCompose.OpenRoomMoveReport, error) {
		moves = append(moves, recordedOpenRoomMove{studentID: studentID, roomID: roomID})
		return report, err
	}, devicescanCompose.OpenRoomRefusals{RoomNotFound: errTestRoomNotFound, RoomNotReleased: errTestRoomNotReleased})
	return mover, &moves
}

func moveChild(t *testing.T, mover devicescanCompose.OpenRoomMover) (devicescan.OpenRoomMoveOutcome, error) {
	t.Helper()
	return mover.MoveToOpenRoom(context.Background(), devicescan.OpenRoomMove{StudentID: testChildID, RoomID: testRoomID})
}

func TestOpenRoomMover_MovesTheChild(t *testing.T) {
	t.Parallel()
	mover, moves := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{RoomSessionID: 250, Moved: []int64{testChildID}}, nil)

	outcome, err := moveChild(t, mover)

	require.NoError(t, err)
	assert.Equal(t, devicescan.OpenRoomMoveOutcome{RoomSessionID: 250, Moved: true}, outcome)
	assert.Equal(t, []recordedOpenRoomMove{{studentID: testChildID, roomID: testRoomID}}, *moves)
}

func TestOpenRoomMover_ChildAlreadyThereIsUnchanged(t *testing.T) {
	t.Parallel()
	mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{RoomSessionID: 250}, nil)

	outcome, err := moveChild(t, mover)

	require.NoError(t, err)
	assert.Equal(t, devicescan.OpenRoomMoveOutcome{RoomSessionID: 250, Moved: false}, outcome)
}

func TestOpenRoomMover_SkippedChild(t *testing.T) {
	t.Parallel()

	t.Run("not present", func(t *testing.T) {
		t.Parallel()
		mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{Skipped: []devicescanCompose.OpenRoomSkip{{StudentID: testChildID, Reason: studentpresence.StudentMoveSkipNotPresent}}}, nil)
		_, err := moveChild(t, mover)
		require.ErrorIs(t, err, devicescan.ErrOpenRoomStudentNotPresent)
	})
	t.Run("conflict", func(t *testing.T) {
		t.Parallel()
		mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{Skipped: []devicescanCompose.OpenRoomSkip{{StudentID: testChildID, Reason: studentpresence.StudentMoveSkipConflict}}}, nil)
		_, err := moveChild(t, mover)
		active, ok := errors.AsType[*devicescan.StudentAlreadyActiveError](err)
		require.True(t, ok, "got %v", err)
		assert.Equal(t, testChildID, active.StudentID)
	})
	t.Run("another child's skip does not concern this one", func(t *testing.T) {
		t.Parallel()
		mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{RoomSessionID: 250, Moved: []int64{testChildID}, Skipped: []devicescanCompose.OpenRoomSkip{{StudentID: testChildID + 1, Reason: studentpresence.StudentMoveSkipConflict}}}, nil)
		outcome, err := moveChild(t, mover)
		require.NoError(t, err)
		assert.True(t, outcome.Moved)
	})
}

func TestOpenRoomMover_TranslatesRefusals(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want error
	}{
		{"unknown room", errTestRoomNotFound, devicescan.ErrOpenRoomNotFound},
		{"room not released", fmt.Errorf("move: %w", errTestRoomNotReleased), devicescan.ErrOpenRoomNotReleased},
		{"child not present", studentpresence.ErrStudentsNotPresent, devicescan.ErrOpenRoomStudentNotPresent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{}, tc.err)
			_, err := moveChild(t, mover)
			require.ErrorIs(t, err, tc.want)
			require.ErrorIs(t, err, tc.err, "the workflow's error stays in the chain for the log")
		})
	}

	t.Run("full room keeps the occupancy", func(t *testing.T) {
		t.Parallel()
		mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{}, &studentpresence.RoomCapacityError{RoomID: testRoomID, RoomName: "Turnhalle", CurrentOccupancy: 20, MaxCapacity: 20})
		_, err := moveChild(t, mover)
		capacity, ok := errors.AsType[*devicescan.RoomCapacityExceededError](err)
		require.True(t, ok, "got %v", err)
		assert.Equal(t, devicescan.RoomCapacityExceededError{RoomID: testRoomID, RoomName: "Turnhalle", CurrentOccupancy: 20, MaxCapacity: 20}, *capacity)
	})
	t.Run("anything else passes through", func(t *testing.T) {
		t.Parallel()
		boom := errors.New("database unavailable")
		mover, _ := openRoomMoverWith(devicescanCompose.OpenRoomMoveReport{}, boom)
		_, err := moveChild(t, mover)
		require.ErrorIs(t, err, boom)
	})
}

func TestNewOpenRoomMover_RequiresTheMoveAndItsRefusals(t *testing.T) {
	t.Parallel()
	move := func(context.Context, int64, int64) (devicescanCompose.OpenRoomMoveReport, error) {
		return devicescanCompose.OpenRoomMoveReport{}, nil
	}
	assert.Panics(t, func() {
		devicescanCompose.NewOpenRoomMover(nil, devicescanCompose.OpenRoomRefusals{RoomNotFound: errTestRoomNotFound, RoomNotReleased: errTestRoomNotReleased})
	})
	assert.Panics(t, func() {
		devicescanCompose.NewOpenRoomMover(move, devicescanCompose.OpenRoomRefusals{RoomNotFound: errTestRoomNotFound})
	})
}
