package compose

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The lifecycle commands authorize the caller on the block and hand the
// transition to the instance lifecycle.

func (s *operations) Start(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.StartedOperation, error) {
	return s.start(ctx, accountID, isAdmin, instanceID, false)
}

func (s *operations) start(ctx context.Context, accountID int64, isAdmin bool, instanceID int64, spontaneous bool) (*timetable.StartedOperation, error) {
	staffID, err := s.requireScopedAction(ctx, accountID, isAdmin, instanceID, ScopedBlockStart)
	if err != nil {
		return nil, err
	}
	if staffID <= 0 {
		return nil, timetable.ErrNoStaffProfile
	}
	return s.deps.Lifecycle.Start(ctx, instanceID, staffID, spontaneous)
}

// CreateAndStartSpontaneous creates and starts an ad-hoc block in the
// caller's request transaction. Create and start share that transaction, so
// either half failing with a non-5xx error must roll back what the other
// half (and the activity resolution before it) wrote: the middleware only
// rolls back on 5xx, so both halves mark the rollback. A create failure is
// wrapped in timetable.SpontaneousCreateError.
func (s *operations) CreateAndStartSpontaneous(ctx context.Context, accountID int64, isAdmin bool, in timetable.SpontaneousStart) (*timetable.StartedOperation, error) {
	instanceID, err := s.deps.Lifecycle.CreateSpontaneous(ctx, in)
	if err != nil {
		tenant.MarkRollback(ctx)
		return nil, &timetable.SpontaneousCreateError{Err: err}
	}
	result, err := s.start(ctx, accountID, isAdmin, instanceID, true)
	if err != nil {
		tenant.MarkRollback(ctx)
		return nil, err
	}
	return result, nil
}

func (s *operations) Complete(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.ScheduledInstance, error) {
	if _, err := s.requireScopedAction(ctx, accountID, isAdmin, instanceID, ScopedBlockComplete); err != nil {
		return nil, err
	}
	return s.deps.Lifecycle.Complete(ctx, instanceID, accountID)
}

// Reopen is gated by the completion actor or an admin: completing a live
// group closes its supervisor row, so requireCanOperate would reject the
// person who just finished the block.
func (s *operations) Reopen(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*timetable.StartedOperation, error) {
	inst, err := s.loadInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if !timetable.CanReopenAsActor(inst.Status == scheduleModels.InstanceStatusCompleted, inst.CompletedBy, accountID, isAdmin) {
		return nil, timetable.ErrTimetableOperationForbidden
	}
	return s.deps.Lifecycle.Reopen(ctx, instanceID, accountID, isAdmin)
}
