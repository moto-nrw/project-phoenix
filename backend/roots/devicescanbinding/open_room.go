package devicescanbinding

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	scanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// NewOpenRoomMover binds the device-scan open-room port to the open-room
// move. It translates the workflow's and Student Presence's refusals into
// the device-scan sentinels the kiosk route classifies.
func NewOpenRoomMover(move scanCompose.OpenRoomMoveFunc, refusals scanCompose.OpenRoomRefusals) devicescan.OpenRoomMover {
	if move == nil || refusals.RoomNotFound == nil || refusals.RoomNotReleased == nil {
		panic("device scan open room mover: the move and both refusals are required")
	}
	return openRoomMover{move: move, refusals: refusals}
}

type openRoomMover struct {
	move     scanCompose.OpenRoomMoveFunc
	refusals scanCompose.OpenRoomRefusals
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
