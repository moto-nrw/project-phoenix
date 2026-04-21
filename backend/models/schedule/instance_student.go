package schedule

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
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

// tableInstanceStudents is the schema-qualified table name.
const tableInstanceStudents = "schedule.instance_students"

// InstanceStudent represents a student's expected attendance at a materialized
// activity instance, plus the three-field attendance model (E18):
//
//   - Status    — system-controlled (expected / present / absent)
//   - Substatus — optional human/auto-set context (late / excused / ...)
//   - Note      — optional freetext up to 500 characters
type InstanceStudent struct {
	base.Model `bun:"schema:schedule,table:instance_students"`
	base.TenantModel

	InstanceID  int64      `bun:"instance_id,notnull" json:"instance_id"`
	StudentID   int64      `bun:"student_id,notnull" json:"student_id"`
	RoomID      *int64     `bun:"room_id" json:"room_id,omitempty"`
	Status      string     `bun:"status,notnull,default:'expected'" json:"status"`
	Substatus   *string    `bun:"substatus" json:"substatus,omitempty"`
	Note        *string    `bun:"note" json:"note,omitempty"`
	CheckedInAt *time.Time `bun:"checked_in_at" json:"checked_in_at,omitempty"`
}

func (s *InstanceStudent) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`schedule.instance_students AS "instance_student"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`schedule.instance_students AS "instance_student"`)
	}
	return nil
}

// TableName returns the database table name.
func (s *InstanceStudent) TableName() string { return tableInstanceStudents }

// GetID implements the Entity interface.
func (s *InstanceStudent) GetID() any { return s.ID }

// GetCreatedAt implements the Entity interface.
func (s *InstanceStudent) GetCreatedAt() time.Time { return s.CreatedAt }

// GetUpdatedAt implements the Entity interface.
func (s *InstanceStudent) GetUpdatedAt() time.Time { return s.UpdatedAt }

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
		return errors.New("note cannot exceed 500 characters")
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

// HasChanges reports whether the patch carries at least one mutation. The
// PATCH handler uses this to reject no-op bodies at 400.
func (p AttendanceFieldPatch) HasChanges() bool {
	return p.Status != nil || p.Substatus != nil || p.SubstatusClear ||
		p.Note != nil || p.NoteClear
}

// InstanceStudentRepository defines operations for managing expected/actual
// attendance on materialized activity instances.
type InstanceStudentRepository interface {
	base.Repository[*InstanceStudent]

	// FindByInstanceID returns all attendance rows for an instance.
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*InstanceStudent, error)

	// FindByStudentAndDateRange returns attendance rows for a student across
	// all instances whose date falls in the inclusive range. Used by the
	// per-student day view (aggregation layer).
	FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to time.Time) ([]*InstanceStudent, error)

	// FindByInstanceAndStudent returns a single attendance row, or nil if the
	// student is not expected at the instance.
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*InstanceStudent, error)

	// DeleteByInstanceID removes all attendance rows for an instance.
	DeleteByInstanceID(ctx context.Context, instanceID int64) error

	// UpdateAttendanceFromCheckin flips status 'expected' → 'present' and
	// stamps checked_in_at. The status='expected' predicate in the WHERE
	// clause enforces monotonicity — a row that's already present (double
	// tap, or post-hoc PATCH) is never clobbered. Returns (updated=true)
	// only when a row was actually changed.
	UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error)

	// UpdateAttendanceFields writes only the fields carried by the patch.
	// Callers (the PATCH handler) must validate cross-field invariants
	// BEFORE invoking — the repo does not re-check them.
	UpdateAttendanceFields(ctx context.Context, id int64, patch AttendanceFieldPatch) error
}
