// Package compose binds the open-room move workflow to the tenant runtime and
// the owner capabilities.
package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	activeSvc "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/internal/application"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove/ports"
)

type Observation = ports.Observation

// RetainedPresence is the part of the retained Student Presence service the
// move binds to until the owner exposes these operations publicly: the room
// session lookup-or-create and the move into it. It is a compatibility
// binding, not a target dependency.
type RetainedPresence interface {
	EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (*active.Group, error)
	MoveStudentsToOpenRoomSessionAuthorized(ctx context.Context, studentIDs []int64, roomSessionID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
}

// Dependencies are the owner capabilities the workflow coordinates.
type Dependencies struct {
	Rooms      ports.Rooms
	Activities ports.Activities
	Presence   RetainedPresence
	Observe    func(Observation)
}

// New binds the workflow. It joins the caller's tenant transaction when one
// is open and otherwise runs the move in its own.
func New(deps Dependencies) (openroommove.Command, error) {
	if deps.Rooms == nil || deps.Activities == nil || deps.Presence == nil || deps.Observe == nil {
		return nil, errors.New("open room move compose: all dependencies are required")
	}
	return application.NewCommand(application.Dependencies{
		Rooms:        deps.Rooms,
		Activities:   deps.Activities,
		RoomSessions: roomSessions{presence: deps.Presence},
		Moves:        moves{presence: deps.Presence},
		Runtime: ports.Runtime{
			TenantID:     tenant.FromContext,
			WithinTenant: tenant.WithinCurrentTenant,
			AcquireLock: func(ctx context.Context, key string) error {
				return tenant.AcquireLock(ctx, key, false)
			},
		},
		Observe: deps.Observe,
	}), nil
}

type roomSessions struct{ presence RetainedPresence }

func (r roomSessions) EnsureOpenRoomSession(ctx context.Context, roomID, activityID int64) (int64, error) {
	session, err := r.presence.EnsureOpenRoomSession(ctx, roomID, activityID)
	if err != nil {
		return 0, err
	}
	return session.ID, nil
}

type moves struct{ presence RetainedPresence }

func (m moves) MoveIntoRoomSession(ctx context.Context, roomSessionID int64, studentIDs []int64, actor openroommove.Actor) (ports.MoveOutcome, error) {
	result, err := m.presence.MoveStudentsToOpenRoomSessionAuthorized(ctx, studentIDs, roomSessionID, activeSvc.StudentMoveAuthorization{
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
