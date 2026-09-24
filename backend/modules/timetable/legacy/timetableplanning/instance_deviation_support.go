package timetableplanning

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The deviation support the retained instance lifecycle keeps until slice S1
// of #3424: the standalone "deliberately unstaffed" acknowledgement, the
// staff move's day lock and error mapping, and the Änderungsprotokoll entries
// of lifecycle writes. The deviation, substitution and sick-report writes
// moved to the Timetable owner (timetable.StaffDeviations, slice S3).

// GuardianNoticeInput is the Timetable owner's cancellation notice text.
type GuardianNoticeInput = timetable.GuardianNoticeInput

// GuardianNoticeResult is the Timetable owner's notice outcome.
type GuardianNoticeResult = timetable.GuardianNoticeResult

func devErrBadRequest(msg string) *timetable.DeviationError {
	return timetable.DeviationBadRequest(msg)
}

func devErrNotFound(msg string) *timetable.DeviationError {
	return timetable.DeviationNotFound(msg)
}

func devErrConflict(code, msg string) *timetable.DeviationError {
	return timetable.DeviationConflict(code, msg)
}

func devErrInternal(operation string, cause error) *timetable.DeviationError {
	return timetable.DeviationInternal(operation, cause)
}

// AcknowledgeUnderstaffed applies the standalone "deliberately unstaffed"
// acknowledgement. It gates past blocks as historical record and serializes
// against same-day staffing saves before delegating to SetUnderstaffedAck; note
// trimming/validation is the caller's (the note arrives already normalized).
func (s *instanceService) AcknowledgeUnderstaffed(ctx context.Context, instanceID int64, ack bool, note *string, actorAccountID *int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	// Past blocks are historical record and read-only. Status alone is
	// insufficient: a materialized past occurrence can still be planned/active,
	// so gate on the Berlin calendar date (#1840).
	if instance.Date.Before(timezone.TodayDate()) {
		return nil, devErrBadRequest("dieser Termin liegt in der Vergangenheit")
	}
	// Serialize with substitute/deviation saves on the same (tenant, date) before
	// validating and writing the acknowledgement (#1840).
	if err := s.acquireSubstituteDayLock(ctx, timezone.Date(instance.Date)); err != nil {
		return nil, devErrInternal("lock day failed", err)
	}
	// Re-read under the lock: a concurrent PUT may have MOVED the block to another
	// day; a move to a past day would bypass the historical-record guard, a move
	// to another day would leave this ack unsynchronized. Detect and reject.
	locked, err := s.loadDeviationInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if locked.Date != instance.Date {
		return nil, devErrConflict("instance_moved", "der Termin wurde gleichzeitig geändert. Öffnen Sie ihn erneut")
	}
	return s.SetUnderstaffedAck(ctx, instanceID, ack, note, actorAccountID)
}

// acquireSubstituteDayLock takes the shared (tenant, date) advisory lock that
// serializes every same-day staffing mutation, within the caller's tx.
func (s *instanceService) acquireSubstituteDayLock(ctx context.Context, date timezone.Date) error {
	return repoBase.AcquireXactLock(ctx, s.deps.DB, substituteDayLockKey(tenant.FromContext(ctx), date))
}

// loadDeviationInstance loads the target instance, mapping the absent/other-
// tenant case to 404 and any other failure to a wrapped 500.
func (s *instanceService) loadDeviationInstance(ctx context.Context, instanceID int64) (*scheduleModel.ActivityInstance, error) {
	instance, err := s.deps.InstanceRepo.FindByID(ctx, instanceID)
	if err != nil {
		// FindByID wraps sql.ErrNoRows in a DatabaseError (never (nil, nil)), so a
		// stale link or deleted/other-tenant instance maps to 404 here.
		if modelBase.IsNoRows(err) {
			return nil, devErrNotFound("der Termin wurde nicht gefunden")
		}
		return nil, devErrInternal("load instance failed", err)
	}
	if instance == nil {
		return nil, devErrNotFound("der Termin wurde nicht gefunden")
	}
	return instance, nil
}

// isPlannableInstance reports whether a substitute/absence write may touch this
// instance. Only planned and active blocks are editable; completed and cancelled
// ones are historical record.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}

// substituteConflictInstance converts an ActivityInstance's TIME columns into
// the minutes-since-midnight form of the owner's conflict probe.
func substituteConflictInstance(inst *scheduleModel.ActivityInstance) timetable.SubstituteConflictInstance {
	return timetable.SubstituteConflictInstance{
		ID:        inst.ID,
		StartMin:  timetable.MinutesOfTime(inst.StartTime.Hour(), inst.StartTime.Minute()),
		EndMin:    timetable.MinutesOfTime(inst.EndTime.Hour(), inst.EndTime.Minute()),
		StartHHMM: inst.StartTime.Format("15:04"),
	}
}

// deviationEventInput bundles one protocol entry before marshalling.
type deviationEventInput struct {
	instance       *scheduleModel.ActivityInstance
	eventType      string
	subjectStaffID *int64
	relatedStaffID *int64
	oldValue       any
	newValue       any
	reason         *string
	actorAccountID *int64
}

// logDeviationEvent appends one audit.deviation_events row inside the caller's
// tenant tx. Fail closed: an error here must abort the surrounding mutation —
// the protocol is the compliance artifact, not a best-effort side channel.
func (s *instanceService) logDeviationEvent(ctx context.Context, in deviationEventInput) error {
	event := &auditModel.DeviationEvent{
		ActivityGroupID: in.instance.ActivityGroupID,
		OccurrenceDate:  auditModel.Date(in.instance.Date),
		StartTime:       timezone.NormalizeWallClock(in.instance.StartTime),
		InstanceID:      &in.instance.ID,
		SubjectStaffID:  in.subjectStaffID,
		RelatedStaffID:  in.relatedStaffID,
		EventType:       in.eventType,
		Reason:          in.reason,
		ActorAccountID:  normalizeActor(in.actorAccountID),
	}
	var err error
	if event.OldValue, err = marshalDeviationValue(in.oldValue); err != nil {
		return &ScheduleError{Op: "log deviation event: marshal old value", Err: err}
	}
	if event.NewValue, err = marshalDeviationValue(in.newValue); err != nil {
		return &ScheduleError{Op: "log deviation event: marshal new value", Err: err}
	}
	if err := s.deps.DeviationEventRepo.Create(ctx, event); err != nil {
		return &ScheduleError{Op: "log deviation event", Err: fmt.Errorf("event_type %s: %w", in.eventType, err)}
	}
	return nil
}

// logSnapshotDropped records a re-plan/split loss: a snapshotted deviation
// whose slot no longer regenerates (#1886). There is no instance to point at —
// the slot anchor (group, date, start_time) is the only identity. One event
// per dropped slot snapshot; old_value carries the full discarded override
// state, new_value stays NULL.
func (s *instanceService) logSnapshotDropped(
	ctx context.Context,
	snap deviationSnapshot,
	targetActivityGroupID *int64,
	actorAccountID *int64,
) error {
	activityGroupID := snap.activityGroupID
	if targetActivityGroupID != nil {
		// Template split: the deviations were snapshotted under the OLD template
		// but would have reapplied onto the successor — anchor the loss there.
		activityGroupID = *targetActivityGroupID
	}
	startTime, err := time.Parse("15:04:05", snap.startTime)
	if err != nil {
		return &ScheduleError{Op: "log dropped deviation: parse start time", Err: err}
	}

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

	event := &auditModel.DeviationEvent{
		ActivityGroupID: &activityGroupID,
		OccurrenceDate:  auditModel.Date(snap.date),
		StartTime:       timezone.NormalizeWallClock(startTime),
		EventType:       auditModel.DeviationEventDroppedByReplan,
		ActorAccountID:  normalizeActor(actorAccountID),
	}
	if event.OldValue, err = marshalDeviationValue(dropped); err != nil {
		return &ScheduleError{Op: "log dropped deviation: marshal", Err: err}
	}
	if err := s.deps.DeviationEventRepo.Create(ctx, event); err != nil {
		return &ScheduleError{Op: "log dropped deviation", Err: err}
	}
	return nil
}

func marshalDeviationValue(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// normalizeActor maps a missing/zero actor to nil so the row stores NULL
// instead of a fabricated id.
func normalizeActor(actorAccountID *int64) *int64 {
	if actorAccountID == nil || *actorAccountID <= 0 {
		return nil
	}
	return actorAccountID
}
