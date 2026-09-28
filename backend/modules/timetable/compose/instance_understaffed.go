package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The "deliberately unstaffed" acknowledgement (Vertretungsplan, #1840): a
// planning annotation that at least one planned position stays unfilled on
// purpose. It touches no session and fires no lifecycle event.

// SetUnderstaffedAck flips the acknowledgement on a planned or active block.
// Clearing it also clears the note, so a stale reason cannot linger.
func (s *InstanceLifecycleService) SetUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*timetable.LifecycleInstance, error) {
	instance, err := s.setUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID, true)
	return LifecycleInstanceOf(instance), err
}

// setUnderstaffedAck implements SetUnderstaffedAck. logEvent=false is the
// re-plan reapply path: reattaching a surviving acknowledgement is not a
// state change, so it writes no protocol entry (#1886, "log only losses").
func (s *InstanceLifecycleService) setUnderstaffedAck(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64, logEvent bool) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusPlanned, scheduleModel.InstanceStatusActive:
		// allowed
	default:
		return nil, fmt.Errorf("%w: cannot acknowledge understaffing on instance in status %q", timetable.ErrInvalidInstanceTransition, instance.Status)
	}
	// Valid whenever the block ends up understaffed; only a fully staffed
	// block (present >= planned) refuses it.
	if ack {
		rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instanceID)
		if err != nil {
			return nil, &ScheduleError{Op: "set understaffed ack: load staff", Err: err}
		}
		if !timetable.IsUnderstaffed(staffingRowsOf(rows)) {
			return nil, timetable.ErrUnderstaffedAckStillStaffed
		}
	}
	previousAck, previousNote := instance.UnderstaffedAck, instance.UnderstaffedNote
	instance.UnderstaffedAck = ack
	instance.UnderstaffedNote = nil
	if ack {
		instance.UnderstaffedNote = note
	}
	if err := s.updateLifecycleColumns(ctx, instance, "understaffed_ack", "understaffed_note"); err != nil {
		return nil, &ScheduleError{Op: "set understaffed ack: update", Err: err}
	}
	// An idempotent replay still writes the columns but records no event.
	if !logEvent || (previousAck == ack && equalOptionalString(previousNote, instance.UnderstaffedNote)) {
		return instance, nil
	}
	eventType := DeviationEventUnderstaffedAck
	if !ack {
		eventType = DeviationEventUnderstaffedUnack
	}
	if err := s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      eventType,
		oldValue:       understaffedAckValue(previousAck, previousNote),
		newValue:       understaffedAckValue(ack, instance.UnderstaffedNote),
		reason:         instance.UnderstaffedNote,
		actorAccountID: actorAccountID,
	}); err != nil {
		return nil, err
	}
	return instance, nil
}

// understaffedAckValue builds the old/new protocol shape of an ack event.
func understaffedAckValue(ack bool, note *string) map[string]any {
	v := map[string]any{"understaffed_ack": ack}
	if note != nil {
		v["note"] = *note
	}
	return v
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// clearStaleAckIfStaffed clears a lingering acknowledgement ONLY when the
// block's staff rows leave it fully staffed. Partial coverage must not
// reopen an acknowledged gap (#1840); it writes only when it clears.
func (s *InstanceLifecycleService) clearStaleAckIfStaffed(ctx context.Context, instance *scheduleModel.ActivityInstance, actorAccountID *int64) error {
	if !instance.UnderstaffedAck {
		return nil
	}
	rows, err := s.deps.InstanceStaffRepo.FindByInstanceID(ctx, instance.ID)
	if err != nil {
		return &ScheduleError{Op: "clear stale ack: load staff", Err: err}
	}
	if timetable.IsUnderstaffed(staffingRowsOf(rows)) {
		return nil // still short-staffed → keep the acknowledgement
	}
	previousNote := instance.UnderstaffedNote
	instance.UnderstaffedAck = false
	instance.UnderstaffedNote = nil
	if err := s.updateLifecycleColumns(ctx, instance, "understaffed_ack", "understaffed_note"); err != nil {
		return &ScheduleError{Op: "clear stale ack: update", Err: err}
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      DeviationEventUnderstaffedUnack,
		oldValue:       understaffedAckValue(true, previousNote),
		newValue:       understaffedAckValue(false, nil),
		actorAccountID: actorAccountID,
	})
}

// ClearUnderstaffedAckIfStaffed clears a lingering acknowledgement only when
// the block is fully staffed now; the substitute flows call it after adding
// coverage (#1840).
func (s *InstanceLifecycleService) ClearUnderstaffedAckIfStaffed(ctx context.Context, instanceID int64, actorAccountID *int64) error {
	instance, err := s.loadForTransition(ctx, instanceID)
	if err != nil {
		return err
	}
	return s.clearStaleAckIfStaffed(ctx, instance, actorAccountID)
}

// AcknowledgeUnderstaffed applies the standalone acknowledgement. Past
// blocks are historical record; the date gate is on the Berlin calendar
// day, because a past occurrence can still be planned. The write serializes
// with same-day staffing saves and refuses a block a concurrent edit moved.
func (s *InstanceLifecycleService) AcknowledgeUnderstaffed(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*timetable.LifecycleInstance, error) {
	instance, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if instance.Date.Before(timezone.TodayDate()) {
		return nil, timetable.DeviationBadRequest(msgInstanceInPast)
	}
	if err := s.acquireSubstituteDayLock(ctx, timezone.Date(instance.Date)); err != nil {
		return nil, timetable.DeviationInternal("lock day failed", err)
	}
	locked, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if locked.Date != instance.Date {
		return nil, timetable.DeviationConflict("instance_moved", msgInstanceMoved)
	}
	return s.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
}

// loadDeviationInstance loads the target block for the staffing writes,
// mapping a missing or other-tenant block to 404 and any other failure to a
// wrapped 500.
func (s *InstanceLifecycleService) loadDeviationInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, timetable.DeviationNotFound(msgInstanceNotFound)
		}
		return nil, timetable.DeviationInternal("load instance failed", err)
	}
	if instance == nil {
		return nil, timetable.DeviationNotFound(msgInstanceNotFound)
	}
	return instance, nil
}
