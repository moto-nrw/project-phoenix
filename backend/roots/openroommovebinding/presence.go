// Package openroommovebinding maps Student Presence commands onto the move workflow's ports.
package openroommovebinding

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/workflows/openroommove"
	"github.com/moto-ogs/project-phoenix/workflows/openroommove/ports"
)

// Presence is the Student Presence move facade the workflow binds to:
// the room session lookup-or-create and the move into it.
type Presence interface {
	PrepareOpenRoomMove(context.Context, []int64, int64) error
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*studentpresence.SessionDetail, error)
	MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth studentpresence.StudentMoveAuthorization) (*studentpresence.StudentMoveResult, error)
}

// CheckIns is the Student Presence check-in into the room session (#6953).
type CheckIns interface {
	CheckInStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID, staffID int64, auth studentpresence.StudentMoveAuthorization, keepPresent bool) (*studentpresence.StudentMoveResult, error)
}

type roomSessions struct{ presence Presence }

func (r roomSessions) EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (int64, error) {
	session, err := r.presence.EnsureOpenRoomSession(ctx, roomID, activityID)
	if err != nil {
		return 0, err
	}
	return session.ID, nil
}

type moves struct {
	Presence
	CheckIns
}

func (m moves) MoveIntoRoomSession(ctx context.Context, roomSessionID int64, studentIDs []int64, actor openroommove.Actor) (ports.MoveOutcome, error) {
	result, err := m.MoveStudentsToOpenRoomSessionAuthorized(ctx, studentIDs, roomSessionID, moveAuthorization(actor))
	if err != nil {
		return ports.MoveOutcome{}, err
	}
	return moveOutcome(result), nil
}

func (m moves) CheckInIntoRoomSession(ctx context.Context, roomSessionID int64, studentIDs []int64, staffID int64, keepPresent bool, actor openroommove.Actor) (ports.MoveOutcome, error) {
	result, err := m.CheckInStudentsToOpenRoomSessionAuthorized(ctx, studentIDs, roomSessionID, staffID, moveAuthorization(actor), keepPresent)
	if err != nil {
		return ports.MoveOutcome{}, err
	}
	return moveOutcome(result), nil
}

func moveAuthorization(actor openroommove.Actor) studentpresence.StudentMoveAuthorization {
	return studentpresence.StudentMoveAuthorization{
		StaffID:                      actor.StaffID,
		BypassResourceChecks:         actor.BypassResourceChecks,
		SchoolWideAttendanceEligible: actor.SchoolWideAttendanceEligible,
	}
}

func moveOutcome(result *studentpresence.StudentMoveResult) ports.MoveOutcome {
	outcome := ports.MoveOutcome{Moved: result.Moved, Unchanged: result.Unchanged, CheckedIn: result.CheckedIn}
	for _, skipped := range result.Skipped {
		outcome.Skipped = append(outcome.Skipped, openroommove.Skipped{StudentID: skipped.StudentID, Reason: skipped.Reason})
	}
	return outcome
}

func NewRoomSessions(presence Presence) ports.RoomSessions {
	if presence == nil {
		return nil
	}
	return roomSessions{presence: presence}
}
func NewMoves(presence Presence, checkIns CheckIns) ports.Moves {
	if presence == nil || checkIns == nil {
		return nil
	}
	return moves{Presence: presence, CheckIns: checkIns}
}
