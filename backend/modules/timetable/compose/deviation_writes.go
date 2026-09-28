package compose

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The row writes behind every deviation save. They were extracted from the
// api/timetable handlers (#1886) so the Änderungsprotokoll is written next to
// every mutation, inside the same tenant transaction.

// substituteWriteOp is one classified substitution target: the instance, the
// absent staff's row on it, and the classified action (one of the
// timetable.SubstituteAction* constants).
type substituteWriteOp struct {
	Instance *scheduleModel.ActivityInstance
	OrigRow  *scheduleModel.InstanceStaff
	Action   string
}

// absenceMarkOptions distinguishes the manual absence write from the #1843
// sick cascade (event type + provenance stamp).
type absenceMarkOptions struct {
	eventType     string
	sickAbsenceID *int64
}

// applyAbsence marks one staff row absent (day-wide semantics; the caller
// loops over all same-day rows) and, for an active instance, ends the absent
// supervisor. Writes an "absence" protocol event in the same tx.
func (s *staffDeviations) applyAbsence(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	reason *string,
	actorAccountID *int64,
	touched timetable.TouchedActivities,
) error {
	return s.applyAbsenceMark(ctx, row, instance, reason, actorAccountID, touched, absenceMarkOptions{
		eventType: DeviationEventAbsence,
	})
}

// MarkSickAbsence is the #1843 sick-cascade variant of the absence write:
// same write semantics, but stamps the provenance id (so deleting the sick
// report can clear exactly these rows) and logs "sick_reported" instead of
// the manual "absence" event.
func (s *staffDeviations) MarkSickAbsence(ctx context.Context, in timetable.SickAbsenceMark, touched timetable.TouchedActivities) error {
	row, instance, err := s.loadAssignment(ctx, in.AssignmentID)
	if err != nil {
		return err
	}
	sickAbsenceID := in.SickAbsenceID
	return s.applyAbsenceMark(ctx, row, instance, in.Reason, in.ActorAccountID, touched, absenceMarkOptions{
		eventType:     DeviationEventSickReported,
		sickAbsenceID: &sickAbsenceID,
	})
}

// loadAssignment loads one staff row and the instance it belongs to.
func (s *staffDeviations) loadAssignment(ctx context.Context, assignmentID int64) (*scheduleModel.InstanceStaff, *scheduleModel.ActivityInstance, error) {
	row, err := s.deps.InstanceStaff.FindByID(ctx, assignmentID)
	if err != nil {
		return nil, nil, fmt.Errorf("load staff assignment %d: %w", assignmentID, err)
	}
	if row == nil {
		return nil, nil, fmt.Errorf("load staff assignment %d: not found", assignmentID)
	}
	instance, err := s.deps.Instances.FindByID(ctx, row.InstanceID)
	if err != nil {
		return nil, nil, fmt.Errorf("load instance %d: %w", row.InstanceID, err)
	}
	if instance == nil {
		return nil, nil, fmt.Errorf("load instance %d: not found", row.InstanceID)
	}
	return row, instance, nil
}

func (s *staffDeviations) applyAbsenceMark(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	reason *string,
	actorAccountID *int64,
	touched timetable.TouchedActivities,
	opts absenceMarkOptions,
) error {
	row.IsAbsent = true
	row.AbsenceReason = reason
	if opts.sickAbsenceID != nil {
		row.SickAbsenceID = opts.sickAbsenceID
	}
	if err := s.deps.InstanceStaff.Update(ctx, row); err != nil {
		return err
	}
	if err := s.endSupervision(ctx, instance, row.StaffID, touched); err != nil {
		return err
	}
	newValue := map[string]any{"is_absent": true}
	if opts.sickAbsenceID != nil {
		newValue["cause"] = "sick"
		newValue["absence_id"] = *opts.sickAbsenceID
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      opts.eventType,
		subjectStaffID: &row.StaffID,
		oldValue:       map[string]any{"is_absent": false},
		newValue:       newValue,
		reason:         reason,
		actorAccountID: actorAccountID,
	})
}

// endSupervision ends the staff member's live supervision of an active block
// and records the block as touched. A planned block has no supervision yet.
func (s *staffDeviations) endSupervision(ctx context.Context, instance *scheduleModel.ActivityInstance, staffID int64, touched timetable.TouchedActivities) error {
	if instance.Status != scheduleModel.InstanceStatusActive || instance.ActiveGroupID == nil {
		return nil
	}
	if _, err := s.deps.Supervisions.EndByActiveGroupAndStaffID(ctx, *instance.ActiveGroupID, staffID); err != nil {
		return err
	}
	touch(touched, instance)
	return nil
}

// applyPresence clears a persisted absence on one row (day-wide semantics).
// Inverse of applyAbsence (#1840). Live supervision on an already-active block
// is restored via the live-session tools, not the planner, so this only flags
// the active group for SSE refetch. Writes a "return_to_presence" event.
func (s *staffDeviations) applyPresence(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	actorAccountID *int64,
	touched timetable.TouchedActivities,
) error {
	previousReason := row.AbsenceReason
	row.IsAbsent = false
	row.AbsenceReason = nil
	// A manual restore also releases any #1843 sick provenance: the row is no
	// longer owed to the sick report, and a later manual re-absence must not
	// be cleared by deleting that report.
	row.SickAbsenceID = nil
	if err := s.deps.InstanceStaff.Update(ctx, row); err != nil {
		return err
	}
	touch(touched, instance)
	oldValue := map[string]any{"is_absent": true}
	if previousReason != nil {
		oldValue["reason"] = *previousReason
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      DeviationEventReturnToPresence,
		subjectStaffID: &row.StaffID,
		oldValue:       oldValue,
		newValue:       map[string]any{"is_absent": false},
		actorAccountID: actorAccountID,
	})
}

// ClearSickAbsence clears one sickness-marked row when its sick report is
// deleted (#1843). Same write semantics as applyPresence, but scoped to the
// cascade: clears the provenance stamp and logs "sick_cleared". Live
// supervision on an already-active block is restored via the live-session
// tools, not here. The block's "deliberately unstaffed" acknowledgement is
// cleared afterwards when the block is fully staffed again.
func (s *staffDeviations) ClearSickAbsence(ctx context.Context, in timetable.SickAbsenceClear, touched timetable.TouchedActivities) error {
	row, instance, err := s.loadAssignment(ctx, in.AssignmentID)
	if err != nil {
		return err
	}
	previousReason := row.AbsenceReason
	row.IsAbsent = false
	row.AbsenceReason = nil
	row.SickAbsenceID = nil
	if err := s.deps.InstanceStaff.Update(ctx, row); err != nil {
		return err
	}
	touch(touched, instance)
	oldValue := map[string]any{"is_absent": true, "cause": "sick", "absence_id": in.SickAbsenceID}
	if previousReason != nil {
		oldValue["reason"] = *previousReason
	}
	if err := s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      DeviationEventSickCleared,
		subjectStaffID: &row.StaffID,
		oldValue:       oldValue,
		newValue:       map[string]any{"is_absent": false},
		actorAccountID: in.ActorAccountID,
	}); err != nil {
		return err
	}
	if err := s.deps.Lifecycle.ClearUnderstaffedAckIfStaffed(ctx, row.InstanceID, in.ActorAccountID); err != nil {
		return fmt.Errorf("reconcile understaffed acknowledgement: %w", err)
	}
	return nil
}

// applySubstitute performs the write for one classified substitution op.
// Shared by the single-day and the multi-day save so the two paths cannot
// diverge. Writes a "substitution" event for every mutating action; the
// idempotent already-substituted action writes nothing and logs nothing.
func (s *staffDeviations) applySubstitute(
	ctx context.Context,
	op substituteWriteOp,
	subID int64,
	reason *string,
	now time.Time,
	actorAccountID *int64,
	touched timetable.TouchedActivities,
) error {
	switch op.Action {
	case timetable.SubstituteActionAlreadyOnInstance:
		// Mark only the absent's original row; the substitute is already a
		// (planned, non-substitute) co-supervisor and is left untouched so
		// reports keep planned co-cover distinct from a Vertretung.
		if err := s.markOriginalAbsent(ctx, op, reason, touched); err != nil {
			return err
		}
		return s.logSubstitutionEvent(ctx, op, subID, reason, actorAccountID, false)
	case timetable.SubstituteActionSubstituted:
		if err := s.markOriginalAbsent(ctx, op, reason, nil); err != nil {
			return err
		}
		if err := s.addSubstitute(ctx, op, subID, now, touched); err != nil {
			return err
		}
		return s.logSubstitutionEvent(ctx, op, subID, reason, actorAccountID, true)
	}
	// Already substituted: the substitute already covers this instance.
	return nil
}

// markOriginalAbsent marks the absent's original row. With touched set it
// also ends the absent person's live supervision; the substitution path ends
// it itself before it starts the substitute's.
func (s *staffDeviations) markOriginalAbsent(ctx context.Context, op substituteWriteOp, reason *string, touched timetable.TouchedActivities) error {
	op.OrigRow.IsAbsent = true
	if reason != nil {
		op.OrigRow.AbsenceReason = reason
	}
	if err := s.deps.InstanceStaff.Update(ctx, op.OrigRow); err != nil {
		return err
	}
	if touched == nil {
		return nil
	}
	return s.endSupervision(ctx, op.Instance, op.OrigRow.StaffID, touched)
}

// addSubstitute inserts the substitute row and, on an active block, hands the
// live supervision from the absent person to the substitute.
func (s *staffDeviations) addSubstitute(ctx context.Context, op substituteWriteOp, subID int64, now time.Time, touched timetable.TouchedActivities) error {
	newRow := &scheduleModel.InstanceStaff{
		InstanceID:   op.Instance.ID,
		StaffID:      subID,
		RoomID:       op.OrigRow.RoomID, // inherit room split, if any
		IsPrimary:    false,
		IsSubstitute: true,
		IsAbsent:     false,
	}
	if err := s.deps.InstanceStaff.Create(ctx, newRow); err != nil {
		return err
	}
	if op.Instance.Status != scheduleModel.InstanceStatusActive || op.Instance.ActiveGroupID == nil {
		return nil
	}
	if _, err := s.deps.Supervisions.EndByActiveGroupAndStaffID(ctx, *op.Instance.ActiveGroupID, op.OrigRow.StaffID); err != nil {
		return err
	}
	newSup := &studentpresence.GroupSupervision{
		StaffID:   subID,
		GroupID:   *op.Instance.ActiveGroupID,
		Role:      "supervisor",
		StartDate: timezone.DateFromTime(now).String(),
	}
	newSup.TenantID = tenant.FromContext(ctx)
	if err := s.deps.Supervisions.CreateSupervision(ctx, newSup); err != nil {
		return err
	}
	touch(touched, op.Instance)
	return nil
}

// removeSubstitute deletes one substitute assignment from one appointment
// without changing that person's other appointments.
func (s *staffDeviations) removeSubstitute(
	ctx context.Context,
	row *scheduleModel.InstanceStaff,
	instance *scheduleModel.ActivityInstance,
	actorAccountID *int64,
	touched timetable.TouchedActivities,
) error {
	if err := s.endSupervision(ctx, instance, row.StaffID, touched); err != nil {
		return err
	}
	if err := s.deps.InstanceStaff.Delete(ctx, row.ID); err != nil {
		return err
	}
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       instance,
		eventType:      DeviationEventSubstituteRemoved,
		subjectStaffID: &row.StaffID,
		oldValue:       map[string]any{"is_substitute": true, "is_absent": row.IsAbsent},
		newValue:       map[string]any{"removed": true},
		actorAccountID: actorAccountID,
	})
}

// logSubstitutionEvent writes the protocol entry for a substitution write.
// rowCreated distinguishes a fresh substitute row from coverage by an
// already-present co-supervisor.
func (s *staffDeviations) logSubstitutionEvent(
	ctx context.Context,
	op substituteWriteOp,
	subID int64,
	reason *string,
	actorAccountID *int64,
	rowCreated bool,
) error {
	return s.logDeviationEvent(ctx, deviationEventInput{
		instance:       op.Instance,
		eventType:      DeviationEventSubstitution,
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
