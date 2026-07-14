// Package schedule — day-wide deviation writes (Vertretungsplan, #1840/#1886).
//
// These methods are the ONLY write path for absence / presence / substitution
// deviations. They were extracted from the api/timetable handlers (#1886) so
// the Änderungsprotokoll (audit.deviation_events) is written next to every
// mutation, inside the same tenant transaction: an audit failure rolls the
// whole mutation back (fail closed). The handlers keep parsing, validation,
// classification, and response shaping; only the writes live here.
package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Substitute action strings — the stable per-instance action vocabulary the
// endpoints return and the write methods switch on. Single source of truth;
// the api/timetable package aliases these.
const (
	SubstituteActionSubstituted       = "substituted"
	SubstituteActionAlreadySubstitute = "already_substituted"
	SubstituteActionAlreadyOnInstance = "already_on_instance"
	SubstituteActionMarkedAbsent      = "marked_absent"
	SubstituteActionAlreadyAbsent     = "already_absent"
	SubstituteActionMarkedPresent     = "marked_present"
)

// SubstituteWriteOp is one classified substitution target crossing the
// handler→service boundary: the instance, the absent staff's row on it, and
// the classified action (one of the SubstituteAction* constants).
type SubstituteWriteOp struct {
	Instance *scheduleModel.ActivityInstance
	OrigRow  *scheduleModel.InstanceStaff
	Action   string
}

// ApplyAbsence marks one staff row absent (day-wide semantics; the caller
// loops over all same-day rows) and, for an active instance, ends the absent
// supervisor. Writes an "absence" protocol event in the same tx.
func (s *instanceService) ApplyAbsence(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	reason *string,
	actorAccountID *int64,
	activeTouched map[int64]*scheduleModel.ActivityInstance,
) error {
	row.IsAbsent = true
	row.AbsenceReason = reason
	if err := s.deps.InstanceStaffRepo.Update(ctx, row); err != nil {
		return err
	}
	if instance.Status == scheduleModel.InstanceStatusActive && instance.ActiveGroupID != nil {
		if _, err := s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, *instance.ActiveGroupID, row.StaffID); err != nil {
			return err
		}
		activeTouched[*instance.ActiveGroupID] = instance
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      auditModel.DeviationEventAbsence,
		subjectStaffID: &row.StaffID,
		oldValue:       map[string]any{"is_absent": false},
		newValue:       map[string]any{"is_absent": true},
		reason:         reason,
		actorAccountID: actorAccountID,
	})
}

// ApplyPresence clears a persisted absence on one row (day-wide semantics).
// Inverse of ApplyAbsence (#1840). Live supervision on an already-active block
// is restored via the live-session tools, not the planner, so this only flags
// the active group for SSE refetch. Writes a "return_to_presence" event.
func (s *instanceService) ApplyPresence(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	actorAccountID *int64,
	activeTouched map[int64]*scheduleModel.ActivityInstance,
) error {
	previousReason := row.AbsenceReason
	row.IsAbsent = false
	row.AbsenceReason = nil
	if err := s.deps.InstanceStaffRepo.Update(ctx, row); err != nil {
		return err
	}
	if instance.Status == scheduleModel.InstanceStatusActive && instance.ActiveGroupID != nil {
		activeTouched[*instance.ActiveGroupID] = instance
	}
	oldValue := map[string]any{"is_absent": true}
	if previousReason != nil {
		oldValue["reason"] = *previousReason
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      auditModel.DeviationEventReturnToPresence,
		subjectStaffID: &row.StaffID,
		oldValue:       oldValue,
		newValue:       map[string]any{"is_absent": false},
		actorAccountID: actorAccountID,
	})
}

// ApplySubstitute performs the write for one classified substitution op.
// Shared by /substitute and /deviations so the two paths cannot diverge.
// Writes a "substitution" event for every mutating action; the idempotent
// already-substituted action writes nothing and logs nothing.
func (s *instanceService) ApplySubstitute(
	ctx context.Context,
	op SubstituteWriteOp,
	subID int64,
	reason *string,
	now time.Time,
	actorAccountID *int64,
	activeTouched map[int64]*scheduleModel.ActivityInstance,
) error {
	switch op.Action {
	case SubstituteActionAlreadySubstitute:
		// No writes: the substitute already covers this instance.
		return nil

	case SubstituteActionAlreadyOnInstance:
		// Mark only the absent's original row; the substitute is already a
		// (planned, non-substitute) co-supervisor and is left untouched so
		// reports keep planned co-cover distinct from a Vertretung.
		op.OrigRow.IsAbsent = true
		if reason != nil {
			op.OrigRow.AbsenceReason = reason
		}
		if err := s.deps.InstanceStaffRepo.Update(ctx, op.OrigRow); err != nil {
			return err
		}
		if op.Instance.Status == scheduleModel.InstanceStatusActive && op.Instance.ActiveGroupID != nil {
			if _, err := s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, *op.Instance.ActiveGroupID, op.OrigRow.StaffID); err != nil {
				return err
			}
			activeTouched[*op.Instance.ActiveGroupID] = op.Instance
		}
		return s.logSubstitutionEvent(ctx, op, subID, reason, actorAccountID, false)

	case SubstituteActionSubstituted:
		op.OrigRow.IsAbsent = true
		if reason != nil {
			op.OrigRow.AbsenceReason = reason
		}
		if err := s.deps.InstanceStaffRepo.Update(ctx, op.OrigRow); err != nil {
			return err
		}
		newRow := &scheduleModel.InstanceStaff{
			InstanceID:   op.Instance.ID,
			StaffID:      subID,
			RoomID:       op.OrigRow.RoomID, // inherit room split, if any
			IsPrimary:    false,
			IsSubstitute: true,
			IsAbsent:     false,
		}
		if err := s.deps.InstanceStaffRepo.Create(ctx, newRow); err != nil {
			return err
		}
		if op.Instance.Status == scheduleModel.InstanceStatusActive && op.Instance.ActiveGroupID != nil {
			if _, err := s.deps.SupervisorRepo.EndByActiveGroupAndStaffID(ctx, *op.Instance.ActiveGroupID, op.OrigRow.StaffID); err != nil {
				return err
			}
			newSup := &activeModel.GroupSupervisor{
				StaffID:   subID,
				GroupID:   *op.Instance.ActiveGroupID,
				Role:      "supervisor",
				StartDate: timezone.DateFromTime(now),
			}
			newSup.SetTenantID(tenant.FromContext(ctx))
			if err := s.deps.SupervisorRepo.Create(ctx, newSup); err != nil {
				return err
			}
			activeTouched[*op.Instance.ActiveGroupID] = op.Instance
		}
		return s.logSubstitutionEvent(ctx, op, subID, reason, actorAccountID, true)
	}
	return nil
}

// logSubstitutionEvent writes the protocol entry for a substitution write.
// rowCreated distinguishes a fresh substitute row from coverage by an
// already-present co-supervisor.
func (s *instanceService) logSubstitutionEvent(
	ctx context.Context,
	op SubstituteWriteOp,
	subID int64,
	reason *string,
	actorAccountID *int64,
	rowCreated bool,
) error {
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       op.Instance,
		eventType:      auditModel.DeviationEventSubstitution,
		subjectStaffID: &op.OrigRow.StaffID,
		relatedStaffID: &subID,
		oldValue:       map[string]any{"is_absent": false},
		newValue: map[string]any{
			"is_absent":              true,
			"substitute_staff_id":    subID,
			"substitute_row_created": rowCreated,
		},
		reason:         reason,
		actorAccountID: actorAccountID,
	})
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
		OccurrenceDate:  in.instance.Date,
		StartTime:       timezone.WallClock(in.instance.StartTime),
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
		OccurrenceDate:  snap.date,
		StartTime:       timezone.WallClock(startTime),
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
