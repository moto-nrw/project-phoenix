package schedulerecords

import (
	"errors"
	"fmt"
	"time"
)

// Attendance status constants (system-controlled, E18).
// Stored as TEXT + CHECK rather than a DB enum so callers can add states
// without an ALTER TYPE.
const (
	AttendanceStatusExpected = "expected"
	AttendanceStatusPresent  = "present"
	AttendanceStatusAbsent   = "absent"
)

// Attendance substatus constants (human- or auto-set, E18). Distinct from
// status: status answers "did the child show up?", substatus captures context.
const (
	AttendanceSubstatusLate      = "late"
	AttendanceSubstatusExcused   = "excused"
	AttendanceSubstatusSick      = "sick"
	AttendanceSubstatusFieldTrip = "field_trip"
	AttendanceSubstatusOther     = "other"
)

// InstanceStudentNoteMaxLength is the maximum length of the freetext `note`
// field on an attendance row.
const InstanceStudentNoteMaxLength = 500

// InstanceStudent represents a student's expected attendance at a materialized
// activity instance, plus the three-field attendance model (E18):
//
//   - Status    — system-controlled (expected / present / absent)
//   - Substatus — optional human/auto-set context (late / excused / ...)
//   - Note      — optional freetext up to 500 characters
//
// It is a composed legacy DTO. schedule.instance_students stores the planned
// participant (InstanceID, StudentID, RoomID); Student Presence stores the
// attendance fields in active.activity_session_attendance, where a missing
// row means expected attendance (#2762). Since the Contract (#2763) the plan
// table has no attendance columns: write and read those fields only through
// the owner commands or the presence projection.
type InstanceStudent struct {
	Model `bun:"schema:schedule,table:instance_students"`
	TenantModel

	InstanceID   int64      `bun:"instance_id,notnull" json:"instance_id"`
	StudentID    int64      `bun:"student_id,notnull" json:"student_id"`
	RoomID       *int64     `bun:"room_id" json:"room_id,omitempty"`
	Status       string     `bun:"status,notnull,default:'expected'" json:"status"`
	Substatus    *string    `bun:"substatus" json:"substatus,omitempty"`
	Note         *string    `bun:"note" json:"note,omitempty"`
	CheckedInAt  *time.Time `bun:"checked_in_at" json:"checked_in_at,omitempty"`
	CheckedOutAt *time.Time `bun:"checked_out_at" json:"checked_out_at,omitempty"`
	IsUnplanned  bool       `bun:"is_unplanned,notnull,default:false" json:"is_unplanned"`
	// NotScheduled records that the care plan did not place this child in the
	// OGS on the instance's date (#1747). It is written once, by the two paths
	// that end a block (instance Complete and the nightly session-end bridge),
	// and frozen from then on: a later care-plan edit must not retroactively
	// rewrite what a finished day meant.
	//
	// It exists as its own column because the fact cannot be inferred from
	// `Status`. Other writers own that column — a PATCH reset writes
	// 'expected', ApplyStatusDay writes 'absent' — and would silently create or
	// destroy the marker. Readers combine both: a row is the non-booking marker
	// only while it is NotScheduled AND still 'expected'; once somebody checks
	// the child in or decides the slot by hand, that decision tells the story.
	NotScheduled bool `bun:"not_scheduled,notnull,default:false" json:"not_scheduled"`
	// ManualStatusAt records that a human set this row's Status by hand through
	// the attendance PATCH endpoint. It is not about WHICH status was chosen —
	// it is the fact that a person chose it.
	//
	// It exists because "still 'expected'" is the population ending a block
	// stamps as a non-booking, and it is also what staff write when they set an
	// unbooked slot back to expected ("this child IS coming, the plan is
	// wrong"). Without this column the completion cannot tell the automatic
	// state from the deliberate one and would relabel the decision as a day the
	// child was never booked, hiding the row from the history and the exports.
	// A row carrying this stamp is never marked NotScheduled; it takes the
	// ordinary expected → absent path like any other genuine expectation.
	ManualStatusAt *time.Time `bun:"manual_status_at" json:"manual_status_at,omitempty"`
	// StudentStatusDayID marks a broad day status that owns the slot absence.
	// Manual slot decisions keep this nil and therefore win over later broad
	// status changes.
	StudentStatusDayID *int64 `bun:"student_status_day_id" json:"-"`
	// PickupExceptionID marks a partial excusal that owns the slot absence.
	// Actual attendance and manual slot decisions clear this provenance.
	PickupExceptionID *int64 `bun:"pickup_exception_id" json:"-"`
}

// PartialAbsenceBlock names one actionable timetable block that a partial
// absence would mark as excused.
type PartialAbsenceBlock struct {
	ID        int64     `bun:"id" json:"id"`
	Title     string    `bun:"title" json:"title"`
	StartTime time.Time `bun:"start_time" json:"start_time"`
	EndTime   time.Time `bun:"end_time" json:"end_time"`
}

// Validate ensures the attendance row is well-formed.
func (s *InstanceStudent) Validate() error {
	if s.InstanceID <= 0 {
		return errors.New("instance_id is required")
	}
	if s.StudentID <= 0 {
		return errors.New("student_id is required")
	}
	if !IsValidAttendanceStatus(s.Status) {
		return errors.New("invalid attendance status")
	}
	if s.Substatus != nil && !IsValidAttendanceSubstatus(*s.Substatus) {
		return errors.New("invalid attendance substatus")
	}
	if s.Note != nil && len(*s.Note) > InstanceStudentNoteMaxLength {
		return fmt.Errorf("note cannot exceed %d characters", InstanceStudentNoteMaxLength)
	}
	if s.RoomID != nil && *s.RoomID <= 0 {
		return errors.New("room_id must be positive when set")
	}
	return nil
}

// IsValidAttendanceStatus reports whether s is a permitted attendance status.
func IsValidAttendanceStatus(s string) bool {
	switch s {
	case AttendanceStatusExpected, AttendanceStatusPresent, AttendanceStatusAbsent:
		return true
	}
	return false
}

// IsValidAttendanceSubstatus reports whether s is a permitted attendance
// substatus (when a substatus is provided at all).
func IsValidAttendanceSubstatus(s string) bool {
	switch s {
	case AttendanceSubstatusLate, AttendanceSubstatusExcused,
		AttendanceSubstatusSick, AttendanceSubstatusFieldTrip,
		AttendanceSubstatusOther:
		return true
	}
	return false
}

// AttendanceFieldPatch carries a targeted update to status/substatus/note.
// A nil pointer means "do not touch"; a non-nil pointer to "" or a valid
// value means "set to this". Substatus and Note additionally carry a
// Clear bool to express "set to NULL" (distinct from "don't touch"), since
// those columns are nullable.
//
// The PATCH handler resolves the JSON tri-state (missing / null / value)
// into this struct before it reaches the repo.
type AttendanceFieldPatch struct {
	Status         *string
	Substatus      *string
	SubstatusClear bool
	Note           *string
	NoteClear      bool
}

// AttendancePatchFieldError describes one field-level attendance patch
// validation failure without tying domain validation to an HTTP package.
type AttendancePatchFieldError struct {
	Field  string
	Reason string
}

// HasChanges reports whether the patch carries at least one mutation. The
// PATCH handler uses this to reject no-op bodies at 400.
func (p AttendanceFieldPatch) HasChanges() bool {
	return p.Status != nil || p.Substatus != nil || p.SubstatusClear ||
		p.Note != nil || p.NoteClear
}

// ParallelPresence reports that a student is recorded status='present' in
// another activity instance that is currently running on the same day. It
// exists so a roster can surface the overlap when consecutive blocks of the
// same lane run in parallel (#2265).
type ParallelPresence struct {
	StudentID  int64
	InstanceID int64
	Title      string
	StartTime  time.Time
	EndTime    time.Time
}

// InstanceStudentKey addresses one instance_students row by its natural key.
// Used by the batch attendance-mirror updates to pass a set of target rows
// into a single composite-IN UPDATE (review #2372).
type InstanceStudentKey struct {
	InstanceID int64
	StudentID  int64
}

// ports.InstanceStudentRecords defines operations for managing expected/actual
// attendance on materialized activity instances.

// StudentInstanceRef is a minimal (student, instance) projection used by the
// timetable retention cleanup's audit bookkeeping.
type StudentInstanceRef struct {
	StudentID  int64 `bun:"student_id"`
	InstanceID int64 `bun:"instance_id"`
}
