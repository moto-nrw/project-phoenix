package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
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
type InstanceStudent struct {
	base.Model `bun:"schema:schedule,table:instance_students"`
	base.TenantModel

	InstanceID   int64      `bun:"instance_id,notnull" json:"instance_id"`
	StudentID    int64      `bun:"student_id,notnull" json:"student_id"`
	RoomID       *int64     `bun:"room_id" json:"room_id,omitempty"`
	Status       string     `bun:"status,notnull,default:'expected'" json:"status"`
	Substatus    *string    `bun:"substatus" json:"substatus,omitempty"`
	Note         *string    `bun:"note" json:"note,omitempty"`
	CheckedInAt  *time.Time `bun:"checked_in_at" json:"checked_in_at,omitempty"`
	CheckedOutAt *time.Time `bun:"checked_out_at" json:"checked_out_at,omitempty"`
	IsUnplanned  bool       `bun:"is_unplanned,notnull,default:false" json:"is_unplanned"`
	// StudentStatusDayID marks a broad day status that owns the slot absence.
	// Manual slot decisions keep this nil and therefore win over later broad
	// status changes.
	StudentStatusDayID *int64 `bun:"student_status_day_id" json:"-"`
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

// InstanceStudentRepository defines operations for managing expected/actual
// attendance on materialized activity instances.
type InstanceStudentRepository interface {
	base.Repository[*InstanceStudent]

	// FindByInstanceID returns all attendance rows for an instance.
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*InstanceStudent, error)

	// FindExpectedByInstanceIDs returns every instance_students row with
	// status='expected' for any of the given instance IDs, tenant-scoped.
	// Returns an empty slice (not nil) when instanceIDs is empty. Used by
	// the WP-B13 exception-conflict endpoint to resolve which students are
	// still flagged as expected on instances that have a cancellation or
	// modification exception attached.
	FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStudent, error)

	// CountNonAbsentByInstanceIDs groups rows by instance_id and returns the
	// count of rows with status != 'absent' per instance, mirroring
	// InstanceStaffRepository.CountNonAbsentByInstanceIDs. Instances with
	// zero non-absent rows do not appear in the returned map - callers must
	// treat missing keys as zero. Feeds the Betreuungsplan capacity
	// computation (children count per block).
	CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error)

	// FindByStudentAndDateRange returns attendance rows for a student across
	// all instances whose date falls in the inclusive range. Used by the
	// per-student day view (aggregation layer).
	FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*InstanceStudent, error)

	// FindByInstanceAndStudent returns a single attendance row, or nil if the
	// student is not expected at the instance.
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*InstanceStudent, error)

	// DeleteByInstanceID removes all attendance rows for an instance.
	DeleteByInstanceID(ctx context.Context, instanceID int64) error

	// UpdateAttendanceFromCheckin opens observed presence for an expected row,
	// a broad-status absence, or a checked-out present row. It stamps the first
	// checked_in_at and clears checked_out_at on re-entry; already-open presence
	// and independent manual decisions are never clobbered. Returns
	// (updated=true) only when a row was actually changed.
	UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error)
	CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (*InstanceStudent, error)
	UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error
	FindCurrentCandidates(ctx context.Context, studentID int64, date timezone.Date, at time.Time) ([]*InstanceStudent, error)
	ApplyStatusDay(ctx context.Context, studentID int64, date timezone.Date, statusDayID int64, substatus string) (int, error)
	ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error)
	ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date timezone.Date) (int, error)

	// UpdateAttendanceFields writes only the fields carried by the patch.
	// Callers (the PATCH handler) must validate cross-field invariants
	// BEFORE invoking — the repo does not re-check them.
	UpdateAttendanceFields(ctx context.Context, id int64, patch AttendanceFieldPatch) error

	// BulkUpdateStatus flips every attendance row for the given instance whose
	// current status equals fromStatus to toStatus, and returns the number of
	// rows changed. Tenant-scoped via the caller's transaction context.
	//
	// Used by Complete() to mark remaining 'expected' students as 'absent'
	// when an instance ends (WP-B10 plan §6.2). Intentionally NOT used by
	// Cancel(): a cancelled instance never ran, so "absent" would be a
	// semantic misclaim.
	BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string) (int, error)

	// FindInstancesWithAttendanceByStudentAndDateRange returns one row per
	// (instance_students, activity_instance) pair for the student across the
	// inclusive date range, sorted by date then start time. Tenant-scoped via
	// the caller's context. Single query — no N+1.
	//
	// Instances the student has no attendance row for (e.g. a spontaneous
	// instance they dropped into without being enrolled) are NOT included;
	// the handler layer enriches those via the visits-side lookup.
	FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*ScheduledInstanceRow, error)

	// FindPlannedStudentIDsByDate returns unique student IDs that have a
	// non-cancelled materialized timetable row on the given date. Used by
	// student search's "kommt heute" heuristic as an additive planning signal.
	FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error)

	// MarkExpectedAbsentByActiveGroupIDs flips status 'expected' → 'absent'
	// for students on still-active instances bridged to the given
	// active.groups. Used by the scheduler's daily session-end bridge.
	MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time) error

	// CloseOpenCheckoutsByActiveGroupIDs stamps checked_out_at on open
	// present rows whose instance is bridged to the given active.groups.
	// Mirrors the daily session-end bulk visit close into slot attendance.
	CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error)

	// ListStudentInstanceRefsBefore returns (student_id, instance_id) pairs
	// for attendance rows whose instance date is before the cutoff, ordered
	// by student then instance. Custom projection (join on activity_instances)
	// the generic filter shape cannot express; feeds the per-student audit
	// rows of the timetable retention cleanup.
	ListStudentInstanceRefsBefore(ctx context.Context, cutoff timezone.Date) ([]StudentInstanceRef, error)
}

// StudentInstanceRef is a minimal (student, instance) projection used by the
// timetable retention cleanup's audit bookkeeping.
type StudentInstanceRef struct {
	StudentID  int64 `bun:"student_id"`
	InstanceID int64 `bun:"instance_id"`
}
