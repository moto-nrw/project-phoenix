package compose

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The lifecycle serves the port the operational day declares for it; the
// composition root hands this view over.

// NewOperationLifecycle is the lifecycle as the operational commands drive it.
func NewOperationLifecycle(s *application.InstanceLifecycleService) application.OperationLifecycle {
	return operationLifecycle{lifecycle: s}
}

type operationLifecycle struct {
	lifecycle *application.InstanceLifecycleService
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
	instance, err := l.lifecycle.CompleteInstanceRow(timetable.WithLifecycleActor(ctx, accountID), instanceID)
	if err != nil || instance == nil {
		return nil, err
	}
	scheduled := application.ScheduledInstanceOf(instance)
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

// SubstituteDayLock takes the day-wide staffing lock (#1840) on the caller's
// transaction, for compositions that serialize with the owner's staffing
// writes (the #1843 sick cascade).
func SubstituteDayLock(db *bun.DB) func(context.Context, calendar.Date) error {
	store := postgres.New(databaseRuntime(db))
	return func(ctx context.Context, date calendar.Date) error {
		return store.AcquireTransactionLock(ctx, timetable.SubstituteDayLockKey(tenant.FromContext(ctx), date))
	}
}
