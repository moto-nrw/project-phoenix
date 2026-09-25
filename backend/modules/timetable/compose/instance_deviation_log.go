package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// The Änderungsprotokoll event types the lifecycle writes record (#1886);
// they match the Audit Platform's deviation event vocabulary.
const (
	DeviationEventCancellation      = "cancellation"
	DeviationEventUnderstaffedAck   = "understaffed_ack"
	DeviationEventUnderstaffedUnack = "understaffed_unack"
	DeviationEventDroppedByReplan   = "deviation_dropped_by_replan"
	DeviationEventDroppedByEdit     = "deviation_dropped_by_edit"
	DeviationEventStaffMoved        = "staff_moved"
)

// logDeviationEvent appends one protocol entry of a lifecycle write inside
// the caller's tenant tx; a failure aborts the write.
func (s *InstanceLifecycleService) logDeviationEvent(ctx context.Context, in deviationEventInput) error {
	return recordDeviationEvent(ctx, s.deps.Protocol, in)
}

// logSnapshotDropped records a re-plan or split loss: a snapshotted
// deviation whose slot no longer regenerates (#1886). The slot anchor
// (group, date, start time) is its only identity; old_value carries the
// whole discarded override state.
func (s *InstanceLifecycleService) logSnapshotDropped(ctx context.Context, snap deviationSnapshot, targetActivityGroupID, actorAccountID *int64) error {
	activityGroupID := snap.activityGroupID
	if targetActivityGroupID != nil {
		// Template split: the deviations were snapshotted under the OLD
		// template but would have reapplied onto the successor.
		activityGroupID = *targetActivityGroupID
	}
	startTime, err := time.Parse("15:04:05", snap.startTime)
	if err != nil {
		return &ScheduleError{Op: "log dropped deviation: parse start time", Err: err}
	}
	event := DeviationEventRecord{
		ActivityGroupID: &activityGroupID,
		OccurrenceDate:  snap.date,
		StartTime:       timezone.NormalizeWallClock(startTime),
		EventType:       DeviationEventDroppedByReplan,
		ActorAccountID:  normalizeActor(actorAccountID),
	}
	if event.OldValue, err = marshalDeviationValue(droppedSnapshotValue(snap)); err != nil {
		return &ScheduleError{Op: "log dropped deviation: marshal", Err: err}
	}
	if err := s.deps.Protocol.RecordDeviationEvent(ctx, event); err != nil {
		return &ScheduleError{Op: "log dropped deviation", Err: err}
	}
	return nil
}

func droppedSnapshotValue(snap deviationSnapshot) map[string]any {
	dropped := map[string]any{
		"understaffed_ack": snap.understaffedAck,
	}
	if snap.understaffedNote != nil {
		dropped["understaffed_note"] = *snap.understaffedNote
	}
	if snap.requiredStaff != nil {
		dropped["required_staff"] = *snap.requiredStaff
	}
	if len(snap.absentPlanned) > 0 {
		absent := make([]map[string]any, 0, len(snap.absentPlanned))
		for _, ab := range snap.absentPlanned {
			entry := map[string]any{"staff_id": ab.staffID}
			if ab.reason != nil {
				entry["reason"] = *ab.reason
			}
			absent = append(absent, entry)
		}
		dropped["absent_planned"] = absent
	}
	if len(snap.substitutes) > 0 {
		subs := make([]map[string]any, 0, len(snap.substitutes))
		for _, sub := range snap.substitutes {
			subs = append(subs, map[string]any{"staff_id": sub.staffID, "is_absent": sub.isAbsent})
		}
		dropped["substitutes"] = subs
	}
	return dropped
}
