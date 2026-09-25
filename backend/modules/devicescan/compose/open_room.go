package compose

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// OpenRoomMover is the device-scan workflow's port to the open-room move
// (#3067), re-exported for the composition root.
type OpenRoomMover = devicescan.OpenRoomMover

// OpenRoomMoveReport is what the open-room move reported for one booking.
type OpenRoomMoveReport struct {
	RoomSessionID int64
	Moved         []int64
	Skipped       []OpenRoomSkip
}

// OpenRoomSkip names a child the move left where it was, and why.
type OpenRoomSkip struct {
	StudentID int64
	Reason    string
}

// OpenRoomMoveFunc runs the open-room move for one child with the kiosk's
// device trust.
type OpenRoomMoveFunc func(ctx context.Context, studentID, roomID int64) (OpenRoomMoveReport, error)

// OpenRoomRefusals names the move workflow's own refusals, which the
// device-scan module cannot import.
type OpenRoomRefusals struct {
	RoomNotFound    error
	RoomNotReleased error
}

// NewOpenRoomMover binds the device-scan open-room port to the open-room
// move. It translates the workflow's and Student Presence's refusals into
// the device-scan sentinels the kiosk route classifies.
func NewOpenRoomMover(move OpenRoomMoveFunc, refusals OpenRoomRefusals) OpenRoomMover {
	if move == nil || refusals.RoomNotFound == nil || refusals.RoomNotReleased == nil {
		panic("device scan open room mover: the move and both refusals are required")
	}
	return openRoomMover{move: move, refusals: refusals}
}

type openRoomMover struct {
	move     OpenRoomMoveFunc
	refusals OpenRoomRefusals
}

func (m openRoomMover) MoveToOpenRoom(ctx context.Context, move devicescan.OpenRoomMove) (devicescan.OpenRoomMoveOutcome, error) {
	report, err := m.move(ctx, move.StudentID, move.RoomID)
	if err != nil {
		return devicescan.OpenRoomMoveOutcome{}, m.translate(err)
	}
	for _, skipped := range report.Skipped {
		if skipped.StudentID != move.StudentID {
			continue
		}
		if skipped.Reason == studentpresence.StudentMoveSkipNotPresent {
			return devicescan.OpenRoomMoveOutcome{}, devicescan.ErrOpenRoomStudentNotPresent
		}
		return devicescan.OpenRoomMoveOutcome{}, &devicescan.StudentAlreadyActiveError{StudentID: move.StudentID}
	}
	return devicescan.OpenRoomMoveOutcome{
		RoomSessionID: report.RoomSessionID,
		Moved:         slices.Contains(report.Moved, move.StudentID),
	}, nil
}

func (m openRoomMover) translate(err error) error {
	if capacity, ok := errors.AsType[*studentpresence.RoomCapacityError](err); ok {
		return &devicescan.RoomCapacityExceededError{
			RoomID:           capacity.RoomID,
			RoomName:         capacity.RoomName,
			CurrentOccupancy: capacity.CurrentOccupancy,
			MaxCapacity:      capacity.MaxCapacity,
		}
	}
	switch {
	case errors.Is(err, m.refusals.RoomNotFound):
		return fmt.Errorf("%w: %w", devicescan.ErrOpenRoomNotFound, err)
	case errors.Is(err, m.refusals.RoomNotReleased):
		return fmt.Errorf("%w: %w", devicescan.ErrOpenRoomNotReleased, err)
	case errors.Is(err, studentpresence.ErrStudentsNotPresent):
		return fmt.Errorf("%w: %w", devicescan.ErrOpenRoomStudentNotPresent, err)
	}
	return err
}
