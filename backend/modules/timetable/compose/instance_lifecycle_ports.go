package compose

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The lifecycle serves the ports the operational day and the deviation
// writes declare for it; the composition root hands these views over.

// OperationLifecycle is the lifecycle as the operational commands drive it.
func (s *InstanceLifecycleService) OperationLifecycle() OperationLifecycle {
	return operationLifecycle{lifecycle: s}
}

// DeviationLifecycle is the lifecycle the deviation saves hand the
// cancellation and the acknowledgement to; its errors pass through
// unchanged.
func (s *InstanceLifecycleService) DeviationLifecycle() DeviationLifecycle {
	return deviationLifecycle{lifecycle: s}
}

type operationLifecycle struct {
	lifecycle *InstanceLifecycleService
}

func (l operationLifecycle) CreateSpontaneous(ctx context.Context, in timetable.SpontaneousStart) (int64, error) {
	spontaneous := true
	instance, err := l.lifecycle.CreateInstance(ctx, timetable.CreateInstanceInput{
		Date:             in.Date,
		StartTime:        in.StartTime,
		EndTime:          in.EndTime,
		Title:            in.Title,
		Description:      in.Description,
		Notes:            in.Notes,
		RoomID:           in.RoomID,
		ActivityGroupID:  in.ActivityGroupID,
		IsSpontaneous:    &spontaneous,
		StaffIDs:         in.StaffIDs,
		CreatedByStaffID: in.CreatedByStaffID,
	})
	if err != nil {
		return 0, err
	}
	if instance == nil {
		return 0, errors.New("create spontaneous instance: no instance returned")
	}
	return instance.ID, nil
}

func (l operationLifecycle) Start(ctx context.Context, instanceID, staffID int64, spontaneous bool) (*timetable.StartedOperation, error) {
	if spontaneous {
		ctx = timetable.WithSpontaneousStartWorkdayGuard(ctx)
	}
	result, err := l.lifecycle.Start(ctx, instanceID, staffID)
	if err != nil {
		return nil, err
	}
	return startedOperation(result), nil
}

func (l operationLifecycle) Complete(ctx context.Context, instanceID, accountID int64) (*timetable.ScheduledInstance, error) {
	instance, err := l.lifecycle.complete(timetable.WithLifecycleActor(ctx, accountID), instanceID)
	if err != nil || instance == nil {
		return nil, err
	}
	scheduled := ScheduledInstanceOf(instance)
	return &scheduled, nil
}

func (l operationLifecycle) Reopen(ctx context.Context, instanceID, accountID int64, isAdmin bool) (*timetable.StartedOperation, error) {
	result, err := l.lifecycle.Reopen(ctx, instanceID, accountID, isAdmin)
	if err != nil {
		return nil, err
	}
	return startedOperation(result), nil
}

func startedOperation(result *timetable.StartInstanceResult) *timetable.StartedOperation {
	if result == nil {
		return nil
	}
	started := &timetable.StartedOperation{ActiveGroupID: result.ActiveGroupID, Warnings: result.Warnings}
	if result.Instance != nil {
		started.InstanceID, started.Status = result.Instance.ID, result.Instance.Status
	}
	return started
}

type deviationLifecycle struct {
	lifecycle *InstanceLifecycleService
}

func (l deviationLifecycle) CancelBlock(ctx context.Context, in DeviationCancellation) (CancelledBlock, error) {
	cancelled, err := l.lifecycle.CancelWithNotice(ctx, timetable.CancelInstanceInput{
		InstanceID: in.InstanceID, Reason: in.Reason, ActorAccountID: in.ActorAccountID, GuardianNotice: in.GuardianNotice,
	})
	if err != nil {
		return CancelledBlock{}, err
	}
	return CancelledBlock{
		InstanceID: cancelled.Instance.ID, UnderstaffedAck: cancelled.Instance.UnderstaffedAck, GuardianNotice: cancelled.GuardianNotice,
	}, nil
}

func (l deviationLifecycle) SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) error {
	_, err := l.lifecycle.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
	return err
}

func (l deviationLifecycle) ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error {
	return l.lifecycle.ClearUnderstaffedAckIfStaffed(ctx, instanceID, actorAccountID)
}

// SubstituteDayLock takes the day-wide staffing lock (#1840) on the caller's
// transaction, for compositions that serialize with the owner's staffing
// writes (the #1843 sick cascade).
func SubstituteDayLock(db *bun.DB) func(context.Context, timezone.Date) error {
	store := postgres.New(databaseRuntime(db))
	return func(ctx context.Context, date timezone.Date) error {
		return store.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenant.FromContext(ctx), date))
	}
}
