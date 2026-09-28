package timetable

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Conflict kinds — stable string values exported to clients so the frontend
// can switch on kind rather than parsing German messages.
//
// There is deliberately no "room" kind (#2139): parallel groups may share a
// room, so a pure room overlap is not a conflict. Only a person (child or
// staff) planned twice at the same time can conflict.
const (
	ConflictKindStaff   = "staff"
	ConflictKindStudent = "student"
	// The two exception-conflict kinds (WP-B13) between an activity exception
	// and a student's arrival expectation.
	ConflictKindCancelledArrivals = "cancelled_instance_with_scheduled_arrivals"
	ConflictKindModifiedMismatch  = "modified_instance_time_mismatch"
)

// InstanceConflictWarning is a single soft warning surfaced on the start
// response and on the calendar's instance list. One entry per (kind,
// resource) pair; several warnings per kind are possible.
type InstanceConflictWarning struct {
	Kind        string `json:"kind"`         // "staff" | "student"
	ResourceID  int64  `json:"resource_id"`  // staff_id / student_id
	Message     string `json:"message"`      // German user-facing copy
	CanOverride bool   `json:"can_override"` // always true in v1

	// The fields below are set by the planning-window detection (#2139) so
	// the calendar can link the two involved blocks and persist a per-user
	// acknowledgement. Start-time warnings (planned vs. live layer) leave
	// them empty — they are transient toast content, not acknowledgeable
	// calendar state.
	Fingerprint           string `json:"fingerprint,omitempty"`
	ConflictingInstanceID int64  `json:"conflicting_instance_id,omitempty"`
	ConflictingTitle      string `json:"conflicting_title,omitempty"`
	OverlapStart          string `json:"overlap_start,omitempty"` // "HH:MM"
	OverlapEnd            string `json:"overlap_end,omitempty"`   // "HH:MM"
}

// StartConflictSubject names the planned block whose start is checked and
// the room it is planned in.
type StartConflictSubject struct {
	InstanceID int64
	RoomID     int64
}

// PlannedConflictProbe is a hypothetical slot (date plus wall-clock window)
// and the resources to check against the day's planned and running blocks.
// At least one of RoomID, StaffIDs or StudentIDs is set; the HTTP handler
// enforces that before probing.
//
// RoomID produces no warnings of its own (#2139: shared rooms are
// sanctioned). It still matters as input: staff double-planning is only a
// conflict when the two slots are not certainly in the same room. A nil
// RoomID means the slot's room is undetermined, so staff overlaps always warn.
type PlannedConflictProbe struct {
	Date              calendar.Date
	StartTime         time.Time // wall clock, any date anchor
	EndTime           time.Time // wall clock, any date anchor
	RoomID            *int64
	StaffIDs          []int64
	StudentIDs        []int64
	ExcludeInstanceID *int64 // block being edited — never conflicts with itself
}

// PlannedConflictWarning is one advisory hit of the planning-time conflict
// probe (GET /api/timetable/conflicts). It names the conflicting block so the
// planner can link to it; there is no can_override flag because the probe
// never blocks anything.
type PlannedConflictWarning struct {
	Kind                  string `json:"kind"` // "staff" | "student"
	ResourceID            int64  `json:"resource_id"`
	Message               string `json:"message"` // German user-facing copy
	ConflictingInstanceID int64  `json:"conflicting_instance_id"`
	ConflictingTitle      string `json:"conflicting_title"`
}

// WindowConflictBlock is one block of the calendar window together with its
// staff and student rows, the unit the window-wide conflict detection (#2139)
// works on. Status is the block's status as the caller read it; only planned
// and running blocks take part. StartTime and EndTime are wall-clock values.
type WindowConflictBlock struct {
	InstanceID int64
	Date       calendar.Date
	Title      string
	StartTime  time.Time
	EndTime    time.Time
	RoomID     int64
	Status     string
	Staff      []WindowConflictStaff
	Students   []WindowConflictStudent
}

// WindowConflictStaff is one staff row of a block. RoomID is the row's
// multi-room override; nil means the block's own room.
type WindowConflictStaff struct {
	StaffID  int64
	RoomID   *int64
	IsAbsent bool
}

// WindowConflictStudent is one student row of a block with its attendance
// status ("expected", "present", "absent").
type WindowConflictStudent struct {
	StudentID int64
	Status    string
}

// ExceptionConflict is one planning conflict between an activity exception
// and a student's arrival expectation. Optional string fields are populated
// per kind and are empty otherwise.
type ExceptionConflict struct {
	Kind               string
	Date               string
	ActivityGroupID    int64
	InstanceID         int64
	ActivityTitle      string
	StudentID          int64
	ExpectedArrival    string
	ArrivalSource      string
	CancellationReason string
	OriginalStartTime  string
	ModifiedStartTime  string
}

// StartConflictQuery checks a planned block against the live layer before it
// starts (WP-B9, #2139). Warnings are advisory; a failure to load the
// expected students or their current presence is returned, because
// unavailable presence is not evidence of no conflict.
type StartConflictQuery interface {
	DetectStartConflicts(context.Context, StartConflictSubject) ([]InstanceConflictWarning, error)
}

// PlanningConflictQuery detects the planning-time conflicts of the
// timetable: a hypothetical slot against the day's blocks, person
// double-bookings inside a calendar window, and activity exceptions that
// collide with student arrivals.
type PlanningConflictQuery interface {
	// DetectPlannedConflicts is best-effort: a failing sub-check degrades to
	// zero warnings of that kind, never to an error.
	DetectPlannedConflicts(context.Context, PlannedConflictProbe) []PlannedConflictWarning
	// DetectWindowConflicts returns the warnings keyed by instance ID. Every
	// warning appears on both involved blocks with the same fingerprint.
	DetectWindowConflicts([]WindowConflictBlock) map[int64][]InstanceConflictWarning
	// DetectExceptionConflicts covers the inclusive [from, to] window and
	// returns an unsorted slice.
	DetectExceptionConflicts(ctx context.Context, from, to calendar.Date) ([]ExceptionConflict, error)
}

// StaffingQuery answers who could cover a block and whether the Dienstplan
// covers planned staff.
type StaffingQuery interface {
	// StaffPoolForInstance categorizes every staff member against the
	// block's window (#1884); an unknown block is ErrActivityInstanceNotFound.
	StaffPoolForInstance(context.Context, int64) (StaffPool, error)
	// DetectShiftCoverage returns the advisory uncovered intervals of the
	// probed staff; invalid probes wrap ErrInvalidShiftCoverageQuery.
	DetectShiftCoverage(context.Context, ShiftCoverageProbe) (ShiftCoverageResult, error)
}

// ConflictDetectionCapability is the conflict detection and staffing
// capability of the Timetable owner.
type ConflictDetectionCapability interface {
	StartConflictQuery
	PlanningConflictQuery
	StaffingQuery
}
