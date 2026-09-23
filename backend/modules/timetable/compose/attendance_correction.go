package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The correction of a completed block's attendance (#2898) behind
// timetable.AttendanceCorrections. It is the write behind POST
// /api/timetable/instances/{id}/students/{id}/correction (schedules:manage)
// and exists next to — not instead of — the ordinary attendance PATCH:
//
//   - PATCH keeps refusing a completed block. A supervisor on duty must not be
//     able to rewrite a finished day, and clients relying on that 409 keep
//     working unchanged.
//   - This path is the deliberate, named intervention: leadership only, reason
//     mandatory, every field change traceable in the Audit Platform's
//     append-only trail.
//
// activity_instances.completion_snapshot is never touched. It records what the
// day meant at the moment it was closed; the correction changes the live row
// and leaves the snapshot as evidence of the original state.

// AttendanceCorrectionRecord is one field change of a correction before the
// trail stores it.
type AttendanceCorrectionRecord struct {
	InstanceID        int64
	StudentID         int64
	ActorAccountID    *int64
	ActorNameSnapshot *string
	FieldName         string
	OldValue          *string
	NewValue          *string
	Reason            string
}

// AttendanceCorrectionTrail is the Audit Platform's append-only correction
// trail, bound at the composition root. Recording runs in the caller's
// transaction, so a failing insert rolls the correction back with it.
type AttendanceCorrectionTrail interface {
	RecordAttendanceCorrections(ctx context.Context, records []AttendanceCorrectionRecord) error
	ListAttendanceCorrections(ctx context.Context, instanceID, studentID int64) ([]timetable.AttendanceCorrectionEntry, error)
}

// AttendanceCorrectionDependencies wires the correction. Participants and
// Instances are required. Without Trail every correction fails closed and
// the trail reads empty. People snapshots the actor's name and Locks
// serializes the write with the block's completion; both are optional in
// read-only test facades.
type AttendanceCorrectionDependencies struct {
	Participants scheduleModel.InstanceStudentRepository
	Instances    scheduleModel.ActivityInstanceRepository
	Trail        AttendanceCorrectionTrail
	People       usersModel.PersonRepository
	Locks        AttendanceLocks
	Logger       *slog.Logger
}

type attendanceCorrections struct {
	deps AttendanceCorrectionDependencies
}

// NewAttendanceCorrections composes the correction of completed blocks.
func NewAttendanceCorrections(deps AttendanceCorrectionDependencies) (timetable.AttendanceCorrections, error) {
	if deps.Participants == nil || deps.Instances == nil {
		return nil, errors.New("timetable attendance corrections: required repository is nil")
	}
	return &attendanceCorrections{deps: deps}, nil
}

func (s *attendanceCorrections) getLogger() *slog.Logger {
	return orDefaultLogger(s.deps.Logger)
}

// CorrectInstanceStudentAttendance changes a child's attendance in a COMPLETED
// activity instance and records every changed field in the trail.
func (s *attendanceCorrections) CorrectInstanceStudentAttendance(
	ctx context.Context,
	instanceID, studentID int64,
	patch timetable.AttendancePatch,
	reason string,
	actorAccountID int64,
) (timetable.CorrectedAttendance, error) {
	if instanceID <= 0 || studentID <= 0 {
		return timetable.CorrectedAttendance{}, timetable.ErrAttendanceEntryNotFound
	}
	reason, err := s.checkCorrectionPreconditions(reason)
	if err != nil {
		return timetable.CorrectedAttendance{}, err
	}
	current, err := s.loadCorrectableEntry(ctx, instanceID, studentID)
	if err != nil {
		return timetable.CorrectedAttendance{}, err
	}

	rowPatch := scheduleModel.AttendanceFieldPatch(patch)
	if verrs := timetable.ValidateAttendancePatch(patch, timetable.SlotAttendance{Status: current.Status, Substatus: current.Substatus}); len(verrs) > 0 {
		return timetable.CorrectedAttendance{}, &timetable.AttendanceValidationError{Fields: verrs}
	}

	rowPatch = changedAttendancePatch(rowPatch, current)
	corrections := s.buildAttendanceCorrections(ctx, instanceID, studentID, rowPatch, current, reason, actorAccountID)
	if len(corrections) == 0 {
		// Every requested field already holds the requested value. Return the
		// row unchanged rather than writing an empty trail entry.
		return correctedAttendanceOf(current), nil
	}
	return s.writeCorrection(ctx, current, rowPatch, corrections)
}

// checkCorrectionPreconditions trims and bounds the mandatory reason and
// fails closed rather than write an untraceable correction.
func (s *attendanceCorrections) checkCorrectionPreconditions(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", timetable.ErrCorrectionReasonRequired
	}
	if len([]rune(reason)) > timetable.AttendanceCorrectionReasonMaxLength {
		return "", timetable.ErrCorrectionReasonTooLong
	}
	if s.deps.Trail == nil {
		return "", timetable.ErrCorrectionTrailUnavailable
	}
	return reason, nil
}

// loadCorrectableEntry loads the child's row, serializes against a
// concurrent Complete/Reopen so the status check and the write describe the
// same instance state, checks that the block is completed and re-reads the
// locked row.
func (s *attendanceCorrections) loadCorrectableEntry(ctx context.Context, instanceID, studentID int64) (*scheduleModel.InstanceStudent, error) {
	current, err := s.deps.Participants.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, timetable.ErrAttendanceEntryNotFound
		}
		return nil, fmt.Errorf("load attendance entry: %w", err)
	}
	if current == nil {
		return nil, timetable.ErrAttendanceEntryNotFound
	}
	if s.deps.Locks != nil {
		if err := s.deps.Locks.LockAttendance(ctx, instanceID); err != nil {
			return nil, fmt.Errorf("lock attendance: %w", err)
		}
	}
	if err := s.requireCompletedInstance(ctx, instanceID); err != nil {
		return nil, err
	}

	// Re-read the row now that it is locked. The load above answers 404 before
	// anything about the instance is reported, but its values may already be
	// stale: whatever wrote between then and the lock would otherwise end up in
	// the trail as this correction's "before" value — the one thing the trail
	// exists to state correctly.
	current, err = s.deps.Participants.FindByInstanceAndStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, fmt.Errorf("reload attendance entry: %w", err)
	}
	if current == nil {
		return nil, timetable.ErrAttendanceEntryNotFound
	}
	return current, nil
}

// requireCompletedInstance accepts only a completed block.
func (s *attendanceCorrections) requireCompletedInstance(ctx context.Context, instanceID int64) error {
	instance, err := s.deps.Instances.FindByID(ctx, instanceID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return timetable.ErrCorrectionInstanceNotFound
		}
		return fmt.Errorf("load instance: %w", err)
	}
	if instance == nil {
		return timetable.ErrCorrectionInstanceNotFound
	}
	switch instance.Status {
	case scheduleModel.InstanceStatusCancelled:
		return timetable.ErrCorrectionCancelled
	case scheduleModel.InstanceStatusCompleted:
		return nil
	default:
		return timetable.ErrCorrectionRequiresCompleted
	}
}

// writeCorrection writes the changed fields and the trail inside the caller's
// transaction: a failing trail insert rolls the correction back with it. A
// correction nobody can trace is worse than a correction that did not happen.
func (s *attendanceCorrections) writeCorrection(
	ctx context.Context,
	current *scheduleModel.InstanceStudent,
	patch scheduleModel.AttendanceFieldPatch,
	corrections []AttendanceCorrectionRecord,
) (timetable.CorrectedAttendance, error) {
	if err := s.deps.Participants.UpdateAttendanceFields(ctx, current.ID, patch); err != nil {
		return timetable.CorrectedAttendance{}, fmt.Errorf("update attendance: %w", err)
	}
	if err := s.deps.Trail.RecordAttendanceCorrections(ctx, corrections); err != nil {
		return timetable.CorrectedAttendance{}, fmt.Errorf("record attendance correction: %w", err)
	}
	updated, err := s.deps.Participants.FindByInstanceAndStudent(ctx, current.InstanceID, current.StudentID)
	if err != nil || updated == nil {
		return timetable.CorrectedAttendance{}, fmt.Errorf("reload corrected attendance: %w", err)
	}

	// GDPR: identifiers only — never the child's name, the note text or the
	// stated reason.
	s.getLogger().Info("attendance corrected",
		slog.Int64("instance_id", current.InstanceID),
		slog.Int64("student_id", current.StudentID),
		slog.Int("changed_fields", len(corrections)),
	)
	return correctedAttendanceOf(updated), nil
}

func correctedAttendanceOf(row *scheduleModel.InstanceStudent) timetable.CorrectedAttendance {
	return timetable.CorrectedAttendance{
		ID:          row.ID,
		InstanceID:  row.InstanceID,
		StudentID:   row.StudentID,
		Status:      row.Status,
		Substatus:   row.Substatus,
		Note:        row.Note,
		CheckedInAt: row.CheckedInAt,
	}
}

// changedAttendancePatch drops submitted values that already match the locked
// row. The status adapter treats every supplied status field as a manual
// decision, so a note-only correction must not carry unchanged fields through.
func changedAttendancePatch(patch scheduleModel.AttendanceFieldPatch, current *scheduleModel.InstanceStudent) scheduleModel.AttendanceFieldPatch {
	if patch.Status != nil && *patch.Status == current.Status {
		patch.Status = nil
	}
	patch.Substatus, patch.SubstatusClear = changedOptionalField(patch.Substatus, patch.SubstatusClear, current.Substatus)
	patch.Note, patch.NoteClear = changedOptionalField(patch.Note, patch.NoteClear, current.Note)
	return patch
}

// changedOptionalField keeps a clear only when there is something to clear
// and a value only when it differs from the current one.
func changedOptionalField(value *string, clear bool, current *string) (*string, bool) {
	if clear {
		return nil, current != nil
	}
	if value != nil && equalStringPtr(value, current) {
		return nil, false
	}
	return value, false
}

// buildAttendanceCorrections turns a patch into one trail row per field that
// actually changes value. A patch that sets a field to what it already holds
// produces no row: the trail records changes, not requests.
func (s *attendanceCorrections) buildAttendanceCorrections(
	ctx context.Context,
	instanceID, studentID int64,
	patch scheduleModel.AttendanceFieldPatch,
	current *scheduleModel.InstanceStudent,
	reason string,
	actorAccountID int64,
) []AttendanceCorrectionRecord {
	var actorID *int64
	if actorAccountID > 0 {
		actorID = &actorAccountID
	}
	actorName := s.resolveActorName(ctx, actorAccountID)

	corrections := make([]AttendanceCorrectionRecord, 0, 3)
	add := func(field string, oldValue, newValue *string) {
		if equalStringPtr(oldValue, newValue) {
			return
		}
		corrections = append(corrections, AttendanceCorrectionRecord{
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
		add(timetable.AttendanceCorrectionFieldStatus, &currentStatus, patch.Status)
	}
	switch {
	case patch.SubstatusClear:
		add(timetable.AttendanceCorrectionFieldSubstatus, current.Substatus, nil)
	case patch.Substatus != nil:
		add(timetable.AttendanceCorrectionFieldSubstatus, current.Substatus, patch.Substatus)
	}
	switch {
	case patch.NoteClear:
		add(timetable.AttendanceCorrectionFieldNote, current.Note, nil)
	case patch.Note != nil:
		add(timetable.AttendanceCorrectionFieldNote, current.Note, patch.Note)
	}
	return corrections
}

// resolveActorName snapshots the acting person's name so the trail survives a
// later account deletion. A missing name is not an error: the account id still
// identifies the actor while the account exists, and the correction itself
// matters more than its label.
func (s *attendanceCorrections) resolveActorName(ctx context.Context, accountID int64) *string {
	if accountID <= 0 || s.deps.People == nil {
		return nil
	}
	person, err := s.deps.People.FindByAccountID(ctx, accountID)
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
// instance, newest first. Returns an empty slice when the trail is not wired
// (read-only test facades).
func (s *attendanceCorrections) GetAttendanceCorrections(ctx context.Context, instanceID, studentID int64) ([]timetable.AttendanceCorrectionEntry, error) {
	if s.deps.Trail == nil {
		return []timetable.AttendanceCorrectionEntry{}, nil
	}
	return s.deps.Trail.ListAttendanceCorrections(ctx, instanceID, studentID)
}
