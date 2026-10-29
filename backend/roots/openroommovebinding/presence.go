// Package openroommovebinding maps Student Presence commands onto the move workflow's ports.
package openroommovebinding

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/ports"
)

// Presence is the Student Presence move facade the workflow binds to:
// the room session lookup-or-create and the move into it.
type Presence interface {
	PrepareOpenRoomMove(context.Context, []int64, int64) error
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*studentpresence.SessionDetail, error)
	MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error)
}

type roomSessions struct{ presence Presence }

func (r roomSessions) EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (int64, error) {
	session, err := r.presence.EnsureOpenRoomSession(ctx, roomID, activityID)
	if err != nil {
		return 0, err
	}
	return session.ID, nil
}

type moves struct{ Presence }

func (m moves) MoveIntoRoomSession(ctx context.Context, roomSessionID int64, studentIDs []int64, actor openroommove.Actor) (ports.MoveOutcome, error) {
	result, err := m.MoveStudentsToOpenRoomSessionAuthorized(ctx, studentIDs, roomSessionID, studentpresence.StudentMoveAuthorization{
		StaffID:                      actor.StaffID,
		BypassResourceChecks:         actor.BypassResourceChecks,
		SchoolWideAttendanceEligible: actor.SchoolWideAttendanceEligible,
	})
	if err != nil {
		return ports.MoveOutcome{}, err
	}
	outcome := ports.MoveOutcome{Moved: result.Moved, Unchanged: result.Unchanged}
	for _, skipped := range result.Skipped {
		outcome.Skipped = append(outcome.Skipped, openroommove.Skipped{StudentID: skipped.StudentID, Reason: skipped.Reason})
	}
	return outcome, nil
}

func NewRoomSessions(presence Presence) ports.RoomSessions {
	if presence == nil {
		return nil
	}
	return roomSessions{presence: presence}
}
func NewMoves(presence Presence) ports.Moves {
	if presence == nil {
		return nil
	}
	return moves{Presence: presence}
}
