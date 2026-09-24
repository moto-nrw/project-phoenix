package timetable

import (
	"context"
	"errors"
	"time"
)

// Attendance corrections (#2898, #3424 slice S3) change the attendance of a
// COMPLETED block and record every changed field in the Audit Platform's
// append-only trail, inside the caller's tenant transaction.

// Sentinels of the attendance correction.
var (
	// ErrAttendanceEntryNotFound is returned when the child has no attendance
	// row in the instance — including the tenant-mismatch case, where RLS
	// makes the row invisible rather than forbidden.
	ErrAttendanceEntryNotFound = errors.New("schedule: attendance entry not found")

	// ErrCorrectionInstanceNotFound is returned when the instance of the
	// attendance row no longer resolves.
	ErrCorrectionInstanceNotFound = errors.New("activity instance not found")

	// ErrCorrectionRequiresCompleted is returned for an instance that is not
	// completed. While the block is planned, running or reopened the ordinary
	// attendance paths apply and no correction trail is written.
	ErrCorrectionRequiresCompleted = errors.New("schedule: only a completed block can be corrected")

	// ErrCorrectionCancelled is returned for a cancelled instance: a block
	// that did not take place has no attendance to correct.
	ErrCorrectionCancelled = errors.New("schedule: attendance of a cancelled block cannot be corrected")

	// ErrCorrectionReasonRequired is returned when the mandatory reason is
	// empty or blank.
	ErrCorrectionReasonRequired = errors.New("schedule: a reason is required to correct attendance")

	// ErrCorrectionReasonTooLong is returned when the reason exceeds
	// AttendanceCorrectionReasonMaxLength.
	ErrCorrectionReasonTooLong = errors.New("schedule: correction reason is too long")

	// ErrCorrectionTrailUnavailable is returned when the trail is not wired.
	// An untraceable correction of a closed record must not be possible, not
	// even through a misconfiguration.
	ErrCorrectionTrailUnavailable = errors.New("schedule: correction trail is not available")
)

// AttendanceCorrectionReasonMaxLength caps the stated reason, counted in
// characters.
const AttendanceCorrectionReasonMaxLength = 500

// The field names of a correction trail entry.
const (
	AttendanceCorrectionFieldStatus    = "status"
	AttendanceCorrectionFieldSubstatus = "substatus"
	AttendanceCorrectionFieldNote      = "note"
)

// CorrectedAttendance is the attendance row after a correction.
type CorrectedAttendance struct {
	ID          int64
	InstanceID  int64
	StudentID   int64
	Status      string
	Substatus   *string
	Note        *string
	CheckedInAt *time.Time
}

// AttendanceCorrectionEntry is one field change of the correction trail. The
// actor's name is snapshotted so the trail survives a later account deletion.
type AttendanceCorrectionEntry struct {
	FieldName   string
	OldValue    *string
	NewValue    *string
	Reason      string
	ActorName   *string
	CorrectedAt time.Time
}

// AttendanceCorrections corrects the attendance of completed blocks.
type AttendanceCorrections interface {
	// CorrectInstanceStudentAttendance changes one child's attendance in a
	// completed block and records every changed field. A patch that changes
	// nothing returns the row unchanged and writes no trail entry.
	CorrectInstanceStudentAttendance(ctx context.Context, instanceID, studentID int64, patch AttendancePatch, reason string, actorAccountID int64) (CorrectedAttendance, error)
	// GetAttendanceCorrections returns one child's trail for one instance,
	// newest first.
	GetAttendanceCorrections(ctx context.Context, instanceID, studentID int64) ([]AttendanceCorrectionEntry, error)
}
