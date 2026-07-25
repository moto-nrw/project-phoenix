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

	// FindByInstanceIDs returns every attendance row (all statuses) for any
	// of the given instance IDs in one query, tenant-scoped. Empty input
	// returns an empty slice without hitting the DB (#1565 list options).
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStudent, error)

	// FindExpectedByInstanceIDs returns every instance_students row with
	// status='expected' for any of the given instance IDs, tenant-scoped.
	// Returns an empty slice (not nil) when instanceIDs is empty. Used by
	// the WP-B13 exception-conflict endpoint to resolve which students are
	// still flagged as expected on instances that have a cancellation or
	// modification exception attached.
	FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStudent, error)

	// FindNotScheduledCandidatesByInstanceIDs returns every row that ending a
	// block may still resolve: rows still 'expected', plus rows a broad day
	// status (sick / excused / class trip) already flipped to 'absent' and
	// still owns (student_status_day_id IS NOT NULL), excluding rows a human
	// decided by hand (manual_status_at). Tenant-scoped, empty slice for empty
	// input.
	//
	// It exists because 'expected' alone is not the candidate set. ApplyStatusDay
	// runs before anything knows whether the child was booked into care that
	// day, so a child who was never booked can already sit there as 'absent'.
	// The session-end bridge has to see those rows too, or MarkNotScheduled
	// never gets the chance to undo that false absence (#1747). The predicate
	// deliberately mirrors MarkNotScheduled's WHERE — a row this query omits is
	// a row that write could not have changed anyway.
	FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*InstanceStudent, error)

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

	// ArchivePlannedByStudentIDsFrom removes still-planned attendance rows for
	// the given students on non-cancelled instances dated on or after `from`,
	// snapshotting each removed row under `transitionID` so the transition's
	// revert can replay it verbatim.
	//
	// Past rows are kept as a historical record, and so is any row that records
	// an actual event — an observed presence or a stamped check-in/checkout.
	// Rows a status day rewrote to 'absent' (planned sickness, excusal, class
	// trip) ARE removed: every timetable and slot-list reader loads instance
	// rows irrespective of status, so leaving them behind keeps a departed child
	// visible and counted (#405 review). The bound is inclusive so a graduation
	// applied during the school day also clears the child's still-planned rows
	// on later blocks of that same day.
	//
	// Used when a grade transition graduates a cohort after the scheduler has
	// already materialized upcoming instances, so departed children stop
	// counting on current and future timetables and staffing ratios.
	// Tenant-scoped; returns the rows removed.
	ArchivePlannedByStudentIDsFrom(ctx context.Context, transitionID int64, studentIDs []int64, from timezone.Date) (int, error)

	// RestoreArchivedByTransition replays the rows
	// ArchivePlannedByStudentIDsFrom removed for `transitionID` (restricted to
	// the given students) and consumes the archive entries. This is what makes a
	// revert the exact inverse of the apply instead of a reconstruction from
	// enrollments, which would resurrect occurrences a supervisor had removed by
	// hand and could never recreate hand-added rows with no enrollment behind
	// them (#405 review). Tenant-scoped; returns the rows restored.
	RestoreArchivedByTransition(ctx context.Context, transitionID int64, studentIDs []int64) (int, error)

	// UpdateAttendanceFromCheckin opens observed presence for an expected row,
	// a broad-status absence, or a checked-out present row. It stamps the first
	// checked_in_at and re-stamps it on re-entry; already-open presence
	// and independent manual decisions are never clobbered. Returns
	// (updated=true) only when a row was actually changed.
	UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error)
	CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (*InstanceStudent, error)
	UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error
	// ReconcileAttendanceInterval replaces the observed interval only when the
	// row still carries the caller's previous check-in and checkout. A missing
	// historical checkout may be repaired for a previously closed visit. The
	// check-in guard protects a later re-entry from an edit to an older visit.
	ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error)
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
	//
	// excludeStudentIDs is skipped by the update. Complete() passes the
	// children the care plan does not place here today (#1747): they were
	// never expected, so recording them absent would be the same kind of
	// misclaim — and it would leak into attendance statistics and exports.
	// Callers must stamp those same children via MarkNotScheduled, or the
	// spared rows carry no record of why they were spared.
	BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludeStudentIDs []int64) (int, error)

	// MarkNotScheduled persists the #1747 non-booking marker on the given
	// (instance, student) rows: the care plan did not place the child in the
	// OGS on that instance's date, so ending the block wrote no absence for
	// them. Rows still 'expected' are stamped as they are. A row a broad day
	// status (sick / excused / class trip) already flipped to 'absent' is
	// stamped too and reset to 'expected', dropping substatus and status-day
	// provenance: those statuses are written before anything knows whether the
	// child was booked, and a child owed no care that day cannot be absent from
	// it. A child who was checked in keeps their outcome, and so does any row
	// carrying manual_status_at — staff can set an unbooked slot back to
	// 'expected', and stamping over that would claim they were never booked.
	//
	// Idempotent, tenant-scoped, and a no-op on an empty slice.
	MarkNotScheduled(ctx context.Context, refs []StudentInstanceRef) error

	// FindInstancesWithAttendanceByStudentAndDateRange returns one row per
	// (instance_students, activity_instance) pair for the student across the
	// inclusive date range, sorted by date then start time. Tenant-scoped via
	// the caller's context. Single query — no N+1.
	//
	// Instances the student has no attendance row for (e.g. a spontaneous
	// instance they dropped into without being enrolled) are NOT included;
	// the handler layer enriches those via the visits-side lookup.
	//
	// Rows carrying the not_scheduled marker while still 'expected' are
	// excluded too: the care plan did not book the child that day (#1747), so
	// reporting them as expected attendance would claim care that never
	// existed. A marked row that somebody later checked in or decided by hand
	// stays visible — that decision outranks the marker. 'Expected' rows on
	// cancelled instances are never marked; the instance status carries that
	// story.
	FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to timezone.Date) ([]*ScheduledInstanceRow, error)

	// HasPlannedSlotsInRange reports whether the tenant has at least one
	// planned slot assignment (is_unplanned = FALSE) on a non-cancelled
	// instance dated within the inclusive range. This is the tenant-level
	// "care plan is used" signal behind the attendance history's assignment
	// hints: a single student's slot rows cannot answer it (a student may
	// simply not be booked anywhere), and walk-in rows must not, because a
	// spontaneous drop-in also happens at schools that plan nothing.
	// Cancelled instances keep their assignment rows but count as no
	// evidence either — a cancelled-only occurrence was never a usable
	// slot. Tenant-scoped via the caller's context.
	HasPlannedSlotsInRange(ctx context.Context, from, to timezone.Date) (bool, error)

	// FindPlannedStudentIDsByDate returns unique student IDs that have a
	// non-cancelled materialized timetable row on the given date. Used by
	// student search's "kommt heute" heuristic as an additive planning signal.
	FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]int64, error)

	// MarkExpectedAbsentByActiveGroupIDs flips status 'expected' → 'absent'
	// for students on still-active instances bridged to the given
	// active.groups. Used by the scheduler's daily session-end bridge.
	//
	// exclusions are (instance, student) pairs skipped by the update,
	// carrying the same rule as BulkUpdateStatus: a child the care plan does
	// not place there THAT day was never expected and must not be recorded
	// absent (#1747), and the same pairs must be stamped via
	// MarkNotScheduled. The pairs are per-instance on purpose — one nightly run
	// can close instances from several dates, and a child not booked on one
	// date may well be booked on another.
	MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []StudentInstanceRef) error

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
