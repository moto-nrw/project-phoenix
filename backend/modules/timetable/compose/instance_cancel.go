package compose

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// deletedSlotReason is stored on the cancellation exception DeleteCancelled
// leaves behind for a deleted materialized occurrence.
const deletedSlotReason = "Einzeltermin gelöscht"

// Cancel implements planned|active → cancelled. A running block's session
// ends the way a completion ends it; a planned block has none yet.
func (s *InstanceLifecycleService) Cancel(ctx context.Context, instanceID int64, reason *string, actorAccountID *int64) (*timetable.LifecycleInstance, error) {
	instance, err := s.cancel(ctx, instanceID, reason, actorAccountID)
	return LifecycleInstanceOf(instance), err
}

func (s *InstanceLifecycleService) cancel(ctx context.Context, instanceID int64, reason *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error) {
	if !s.hasTx(ctx) {
		var result *scheduleModel.ActivityInstance
		err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
			var cancelErr error
			result, cancelErr = s.cancel(txCtx, instanceID, reason, actorAccountID)
			return cancelErr
		})
		return result, err
	}
	instance, err := s.cancellableInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if err := s.lockCancellationState(ctx, instance); err != nil {
		return nil, err
	}
	if instance.Status == scheduleModel.InstanceStatusActive {
		if err := s.deps.ActiveService.EndActivitySession(ctx, *instance.ActiveGroupID); err != nil {
			return nil, &ScheduleError{Op: "cancel instance: end active.group", Err: err}
		}
	}
	previousStatus := instance.Status
	now := time.Now()
	instance.Status = scheduleModel.InstanceStatusCancelled
	instance.CompletedAt = &now
	instance.CancelReason = reason
	if err := s.updateLifecycleColumns(ctx, instance, "status", "completed_at", "cancel_reason"); err != nil {
		return nil, &ScheduleError{Op: "cancel instance: update", Err: err}
	}
	if err := s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      DeviationEventCancellation,
		oldValue:       map[string]any{"status": previousStatus},
		newValue:       map[string]any{"status": scheduleModel.InstanceStatusCancelled},
		reason:         reason,
		actorAccountID: actorAccountID,
	}); err != nil {
		return nil, err
	}
	s.broadcastInstanceEvent(ctx, LifecycleEventInstanceCancelled, instance, nil, nil)
	return instance, nil
}

// cancellableInstance loads a planned or running block under its day lock.
// POST /instances/{id}/cancel reaches the lifecycle directly, so without the
// lock a cancel could commit between another admin's lock and their staff
// writes, leaving a cancelled block with rewritten staffing (#1840). The
// lock is re-entrant for the deviations save's cancel branch.
func (s *InstanceLifecycleService) cancellableInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	instance, err = s.lockDayAndReload(ctx, instance, "cancel instance")
	if err != nil {
		return nil, err
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusActive:
		return instance, nil
	default:
		return nil, fmt.Errorf("%w: cannot cancel instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
}

// lockCancellationState takes the session lock of a running block before
// the attendance row locks — the order a completion and a kiosk session end
// take — and then the attendance locks the attendance patch takes, so a
// patch cannot commit after the cancel froze the block.
func (s *InstanceLifecycleService) lockCancellationState(ctx context.Context, instance *scheduleModel.ActivityInstance) error {
	if instance.Status == scheduleModel.InstanceStatusActive {
		if instance.ActiveGroupID == nil {
			// A running block without a session is data corruption; abort
			// rather than "cancel" a session that never ran.
			return &ScheduleError{Op: "cancel instance", Err: fmt.Errorf("active instance %d has no active_group_id", instance.ID)}
		}
		group, err := s.deps.ActiveGroupRepo.FindByIDForUpdate(ctx, *instance.ActiveGroupID)
		if err != nil {
			return &ScheduleError{Op: "cancel instance: lock group", Err: err}
		}
		if group == nil || group.EndTime != nil {
			return fmt.Errorf("%w: active group is not open", timetable.ErrInvalidInstanceTransition)
		}
	}
	if s.deps.RecoveryRepo != nil {
		if err := s.deps.RecoveryRepo.LockAttendance(ctx, instance.ID); err != nil {
			return &ScheduleError{Op: "cancel instance: lock attendance", Err: err}
		}
	}
	return nil
}

// DeleteCancelled permanently removes a planned or cancelled block (the
// historical name stays for the handlers). Running and completed blocks
// stay protected. A materialized occurrence first gets a cancelled
// exception, so the materialization cannot resurrect it.
func (s *InstanceLifecycleService) DeleteCancelled(ctx context.Context, instanceID int64) error {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return err
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusCancelled:
		// allowed
	default:
		return fmt.Errorf("%w: cannot delete instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	if instance.ActivityGroupID != nil && !instance.IsSpontaneous {
		if err := s.rejectAmbiguousTemplateDelete(ctx, *instance.ActivityGroupID, timezone.Date(instance.Date)); err != nil {
			return err
		}
		if err := s.ensureCancelledSlotException(ctx, *instance.ActivityGroupID, timezone.Date(instance.Date), deletedSlotReason); err != nil {
			return err
		}
	}
	if err := s.deps.InstanceRepo.Delete(ctx, instance.ID); err != nil {
		return &ScheduleError{Op: "delete instance", Err: err}
	}
	s.getLogger().Info("instance deleted",
		slog.Int64("tenant_id", tenant.FromContext(ctx)),
		slog.Int64("instance_id", instance.ID),
		slog.String("date", instance.Date.String()),
		slog.String("status", instance.Status),
	)
	s.broadcastPlannedInstanceChanged(ctx, "instance_delete")
	return nil
}

// rejectAmbiguousTemplateDelete refuses the delete when the template has
// several materialized slots that date: the cancellation exception is
// date-wide and would consume all of them.
func (s *InstanceLifecycleService) rejectAmbiguousTemplateDelete(ctx context.Context, activityGroupID int64, date timezone.Date) error {
	rows, err := s.deps.InstanceRepo.FindByActivityGroupAndDate(ctx, activityGroupID, scheduleModel.Date(date))
	if err != nil {
		return &ScheduleError{Op: "delete instance: check same-day template slots", Err: err}
	}
	templateBacked := 0
	for _, row := range rows {
		if row != nil && !row.IsSpontaneous {
			templateBacked++
		}
	}
	if templateBacked > 1 {
		return fmt.Errorf("%w: template has %d same-day slots", timetable.ErrAmbiguousTemplateInstanceDelete, templateBacked)
	}
	return nil
}

// ensureCancelledSlotException consumes the slot with a cancelled
// exception, converting an existing modified one.
func (s *InstanceLifecycleService) ensureCancelledSlotException(ctx context.Context, activityGroupID int64, date timezone.Date, reason string) error {
	existing, err := s.deps.ExceptionRepo.FindByActivityGroupAndDate(ctx, activityGroupID, scheduleModel.Date(date))
	if err != nil {
		return &ScheduleError{Op: "delete instance: check slot exception", Err: err}
	}
	if existing != nil {
		if existing.ExceptionType == scheduleModel.ActivityExceptionCancelled {
			return nil
		}
		existing.ExceptionType = scheduleModel.ActivityExceptionCancelled
		existing.StartTime = nil
		existing.EndTime = nil
		existing.RoomID = nil
		existing.Reason = &reason
		if err := s.deps.ExceptionRepo.Update(ctx, existing); err != nil {
			return &ScheduleError{Op: "delete instance: cancel existing exception", Err: err}
		}
		s.getLogger().Info("deleted instance: existing exception converted to cancellation",
			slog.Int64("activity_group_id", activityGroupID),
			slog.String("date", date.String()),
		)
		return nil
	}
	exc := &scheduleModel.ActivityException{
		ActivityGroupID: activityGroupID,
		ExceptionDate:   scheduleModel.Date(date),
		ExceptionType:   scheduleModel.ActivityExceptionCancelled,
		Reason:          &reason,
	}
	exc.SetTenantID(tenant.FromContext(ctx))
	if err := s.deps.ExceptionRepo.Create(ctx, exc); err != nil {
		return &ScheduleError{Op: "delete instance: create cancellation exception", Err: err}
	}
	s.getLogger().Info("deleted instance: slot consumed via cancelled exception",
		slog.Int64("activity_group_id", activityGroupID),
		slog.String("date", date.String()),
	)
	return nil
}
