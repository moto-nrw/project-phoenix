package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// Sentinels for the attendance correction path (#2898).
var (
	// ErrAttendanceEntryNotFound is returned when the child has no attendance
	// row in the instance — including the tenant-mismatch case, where RLS
	// makes the row invisible rather than forbidden.
	ErrAttendanceEntryNotFound = errors.New("schedule: attendance entry not found")

	// ErrCorrectionRequiresCompleted is returned for an instance that is not
	// completed. A correction is by definition an intervention into a closed
	// record; while the block is planned, running or reopened the ordinary
	// attendance paths apply and no correction trail should be written.
	ErrCorrectionRequiresCompleted = errors.New("schedule: only a completed block can be corrected")

	// ErrCorrectionCancelled is returned for a cancelled instance. A cancelled
	// block did not take place, so there is no attendance to correct —
	// allowing it would let someone record a child as present at a block that
	// never happened.
	ErrCorrectionCancelled = errors.New("schedule: attendance of a cancelled block cannot be corrected")

	// ErrCorrectionReasonRequired is returned when the mandatory reason is
	// empty or blank.
	ErrCorrectionReasonRequired = errors.New("schedule: a reason is required to correct attendance")

	// ErrCorrectionReasonTooLong is returned when the reason exceeds
	// auditModel.CorrectionReasonMaxLength.
	ErrCorrectionReasonTooLong = errors.New("schedule: correction reason is too long")

	// ErrCorrectionTrailUnavailable is returned when the audit repository is
	// not wired. Failing closed is deliberate: an untraceable correction of a
	// closed record must not be possible, not even through a misconfiguration.
	ErrCorrectionTrailUnavailable = errors.New("schedule: correction trail is not available")
)

// CorrectInstanceStudentAttendance changes a child's attendance in a COMPLETED
// activity instance and records every changed field in the append-only trail
// audit.attendance_corrections.
//
// It is the write behind POST
// /api/timetable/instances/{id}/students/{id}/correction (schedules:manage) and
// exists next to — not instead of — the ordinary attendance PATCH:
//
//   - PATCH keeps refusing a completed block. A supervisor on duty must not be
//     able to rewrite a finished day, and clients relying on that 409 keep
//     working unchanged.
//   - This path is the deliberate, named intervention: leadership only, reason
//     mandatory, every field change traceable.
//
// activity_instances.completion_snapshot is never touched. It records what the
// day meant at the moment it was closed; the correction changes the live row
// and leaves the snapshot as evidence of the original state.
func (s *TimetableDataService) CorrectInstanceStudentAttendance(
	ctx context.Context,
	instanceID, studentID int64,
	patch scheduleModel.AttendanceFieldPatch,
	reason string,
	actorAccountID int64,
) (*scheduleModel.InstanceStudent, error) {
	if instanceID <= 0 || studentID <= 0 {
		return nil, ErrAttendanceEntryNotFound
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrCorrectionReasonRequired
	}
	if len([]rune(reason)) > auditModel.CorrectionReasonMaxLength {
		return nil, ErrCorrectionReasonTooLong
	}
	// Fail closed rather than write an untraceable correction.
	if s.deps.AttendanceCorrectionRepo == nil {
		return nil, ErrCorrectionTrailUnavailable
	}

	current, err := s.GetInstanceStudent(ctx, instanceID, studentID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrAttendanceEntryNotFound
		}
		return nil, fmt.Errorf("load attendance entry: %w", err)
	}
	if current == nil {
		return nil, ErrAttendanceEntryNotFound
	}

	// Serialize against a concurrent Complete/Reopen so the status check and
	// the write describe the same instance state.
	if err := s.LockInstanceAttendance(ctx, instanceID); err != nil {
		return nil, fmt.Errorf("lock attendance: %w", err)
	}

	instance, err := s.GetActivityInstance(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrInstanceNotFound
		}
		return nil, fmt.Errorf("load instance: %w", err)
	}
	if instance == nil {
		return nil, ErrInstanceNotFound
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusCancelled:
		return nil, ErrCorrectionCancelled
	case scheduleModel.InstanceStatusCompleted:
		// The only correctable state.
	default:
		return nil, ErrCorrectionRequiresCompleted
	}

	// Re-read the row now that it is locked. The load above answers 404 before
	// anything about the instance is reported, but its values may already be
	// stale: whatever wrote between then and the lock would otherwise end up in
	// the trail as this correction's "before" value — the one thing this table
	// exists to state correctly.
	current, err = s.GetInstanceStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, fmt.Errorf("reload attendance entry: %w", err)
	}
	if current == nil {
		return nil, ErrAttendanceEntryNotFound
	}

	if verrs := ValidateAttendancePatch(patch, current); len(verrs) > 0 {
		return nil, &TimetableAttendanceValidationError{Fields: verrs}
	}

	corrections := s.buildAttendanceCorrections(ctx, instanceID, studentID, patch, current, reason, actorAccountID)
	if len(corrections) == 0 {
		// Every requested field already holds the requested value. Return the
		// row unchanged rather than writing an empty trail entry.
		return current, nil
	}

	if err := s.deps.InstanceStudentRepo.UpdateAttendanceFields(ctx, current.ID, patch); err != nil {
		return nil, fmt.Errorf("update attendance: %w", err)
	}

	// The trail is written inside the caller's transaction: a failing audit
	// insert rolls the correction back with it. A correction nobody can trace
	// is worse than a correction that did not happen.
	if err := s.deps.AttendanceCorrectionRepo.CreateBatch(ctx, corrections); err != nil {
		return nil, fmt.Errorf("record attendance correction: %w", err)
	}

	updated, err := s.GetInstanceStudent(ctx, instanceID, studentID)
	if err != nil || updated == nil {
		return nil, fmt.Errorf("reload corrected attendance: %w", err)
	}

	// GDPR: identifiers only — never the child's name, the note text or the
	// stated reason.
	s.getLogger().Info("attendance corrected",
		slog.Int64("instance_id", instanceID),
		slog.Int64("student_id", studentID),
		slog.Int("changed_fields", len(corrections)),
	)
	return updated, nil
}

// buildAttendanceCorrections turns a patch into one audit row per field that
// actually changes value. A patch that sets a field to what it already holds
// produces no row: the trail records changes, not requests.
func (s *TimetableDataService) buildAttendanceCorrections(
	ctx context.Context,
	instanceID, studentID int64,
	patch scheduleModel.AttendanceFieldPatch,
	current *scheduleModel.InstanceStudent,
	reason string,
	actorAccountID int64,
) []*auditModel.AttendanceCorrection {
	var actorID *int64
	if actorAccountID > 0 {
		actorID = &actorAccountID
	}
	actorName := s.resolveActorName(ctx, actorAccountID)

	corrections := make([]*auditModel.AttendanceCorrection, 0, 3)
	add := func(field string, oldValue, newValue *string) {
		if equalStringPtr(oldValue, newValue) {
			return
		}
		corrections = append(corrections, &auditModel.AttendanceCorrection{
			InstanceID:        instanceID,
			StudentID:         studentID,
			ActorAccountID:    actorID,
			ActorNameSnapshot: actorName,
			FieldName:         field,
			OldValue:          oldValue,
			NewValue:          newValue,
			Reason:            reason,
		})
	}

	if patch.Status != nil {
		currentStatus := current.Status
		add(auditModel.AttendanceFieldStatus, &currentStatus, patch.Status)
	}
	switch {
	case patch.SubstatusClear:
		add(auditModel.AttendanceFieldSubstatus, current.Substatus, nil)
	case patch.Substatus != nil:
		add(auditModel.AttendanceFieldSubstatus, current.Substatus, patch.Substatus)
	}
	switch {
	case patch.NoteClear:
		add(auditModel.AttendanceFieldNote, current.Note, nil)
	case patch.Note != nil:
		add(auditModel.AttendanceFieldNote, current.Note, patch.Note)
	}
	return corrections
}

// resolveActorName snapshots the acting person's name so the trail survives a
// later account deletion. A missing name is not an error: the account id still
// identifies the actor while the account exists, and the correction itself
// matters more than its label.
func (s *TimetableDataService) resolveActorName(ctx context.Context, accountID int64) *string {
	if accountID <= 0 || s.deps.PersonRepo == nil {
		return nil
	}
	person, err := s.deps.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return nil
	}
	name := strings.TrimSpace(person.GetFullName())
	if name == "" {
		return nil
	}
	return &name
}

func equalStringPtr(a, b *string) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return *a == *b
	}
}

// GetAttendanceCorrections returns one child's correction trail for one
// instance, newest first. Returns an empty slice when the trail repository is
// not wired (read-only test facades).
func (s *TimetableDataService) GetAttendanceCorrections(ctx context.Context, instanceID, studentID int64) ([]*auditModel.AttendanceCorrection, error) {
	if s.deps.AttendanceCorrectionRepo == nil {
		return []*auditModel.AttendanceCorrection{}, nil
	}
	return s.deps.AttendanceCorrectionRepo.ListByInstanceAndStudent(ctx, instanceID, studentID)
}
