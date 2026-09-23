package timetable

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Operational answers of the "Heute geplant" views, the supervision rosters
// and the lifecycle actions staff take on a running day.
var (
	ErrTimetableOperationForbidden = errors.New("timetable operation forbidden")
	ErrTimetableOperationNotFound  = errors.New("timetable operation not found")
	ErrTimetableOperationConflict  = errors.New("timetable operation conflict")
	// ErrNoStaffProfile separates an admin without a staff record — who may
	// act on the block but cannot be recorded as the acting staff member —
	// from a caller who is not planned for it (#3167).
	ErrNoStaffProfile = fmt.Errorf("%w: no staff profile", ErrTimetableOperationForbidden)
)

// Execution states the operational views show beside the planning states:
// a block runs in, or has ended with, its Student Presence session.
const (
	InstanceStatusActive    = "active"
	InstanceStatusCompleted = "completed"
)

// PlannedNowScopePast flips PlannedNow to the complement of its default
// window (#2335): today's finished blocks — completed ones, and planned ones
// whose end time has passed without ever being started.
const PlannedNowScopePast = "past"

// PlannedNowScopeDay returns the day's blocks in every lifecycle state with no
// time window (#2527). Who sees which blocks follows the operational-overview
// rule of the default scope (#2383); school-portal tokens always collapse to
// their own assignments.
const PlannedNowScopeDay = "day"

// PlannedNowOptions narrows the planned-now read.
type PlannedNowOptions struct {
	HorizonMinutes int
	Limit          int
	IncludeRoster  bool
	// Scope is "" for the default upcoming window, PlannedNowScopePast or
	// PlannedNowScopeDay.
	Scope string
}

// OperationPlannedInstance is one block of the caller's day plan.
type OperationPlannedInstance struct {
	ID                    int64   `json:"id"`
	Title                 string  `json:"title"`
	Date                  string  `json:"date"`
	StartTime             string  `json:"start_time"`
	EndTime               string  `json:"end_time"`
	RoomID                int64   `json:"room_id"`
	RoomName              *string `json:"room_name,omitempty"`
	Status                string  `json:"status"`
	IsOverdue             bool    `json:"is_overdue"`
	MinutesUntilStart     int     `json:"minutes_until_start"`
	ExpectedStudentsCount int     `json:"expected_students_count"`
	PresentStudentsCount  int     `json:"present_students_count"`
	// NotScheduledCount is how many assigned children are not in care here
	// today (#1747); they are left out of ExpectedStudentsCount.
	NotScheduledCount   int                       `json:"not_scheduled_students_count"`
	AssignedStaffIDs    []int64                   `json:"assigned_staff_ids"`
	IsAssigned          bool                      `json:"is_assigned"`
	IsPrimary           bool                      `json:"is_primary"`
	IsSubstitute        bool                      `json:"is_substitute"`
	IsAbsent            bool                      `json:"is_absent"`
	RosterPreview       []OperationRosterRow      `json:"roster_preview,omitempty"`
	PickupTimesLoaded   bool                      `json:"pickup_times_loaded"`
	PickupTimesRedacted bool                      `json:"pickup_times_redacted,omitempty"`
	Warnings            []InstanceConflictWarning `json:"warnings"`
	CanStart            bool                      `json:"can_start"`
	StartAvailableAt    string                    `json:"start_available_at"`
	StartExpiresAt      string                    `json:"start_expires_at"`
	// ActiveGroupID is the live session behind a running block (#2383).
	ActiveGroupID *int64 `json:"active_group_id,omitempty"`
	// CancelReason explains a cancelled block ("fällt aus: …", #1840).
	CancelReason *string `json:"cancel_reason,omitempty"`
	// PlanningTrackName/Color carry the planned colour coding into the
	// whole-day scope (#2383). Nil outside scope=day and without a track.
	PlanningTrackName  *string `json:"planning_track_name,omitempty"`
	PlanningTrackColor *string `json:"planning_track_color,omitempty"`
	// GroupName is the education group the block's template targets. Nil
	// outside scope=day and for blocks without a template group.
	GroupName *string `json:"group_name,omitempty"`
	// StaffNames lists the assigned (non-absent) staff. Empty outside
	// scope=day.
	StaffNames []OperationStaffName `json:"staff_names,omitempty"`
}

// OperationStaffName is one assigned staff member on a day-scope block.
type OperationStaffName struct {
	StaffID      int64  `json:"staff_id"`
	DisplayName  string `json:"display_name"`
	IsSubstitute bool   `json:"is_substitute"`
}

// OperationActiveSession is one running block seen from its live session
// (#2265). StartTime/EndTime are the PLAN window ("15:04").
type OperationActiveSession struct {
	ActiveGroupID int64  `json:"active_group_id"`
	InstanceID    int64  `json:"instance_id"`
	Title         string `json:"title"`
	StartTime     string `json:"start_time"`
	EndTime       string `json:"end_time"`
}

// OperationSessionBlock is the block running behind one live session, seen
// by the caller (#3281). StartTime/EndTime are the plan window.
type OperationSessionBlock struct {
	ActiveGroupID int64
	InstanceID    int64
	Title         string
	StartTime     string
	EndTime       string
	// IsAssigned reports a plan entry of the caller that is not absent.
	IsAssigned bool
	// CanOperate reports whether the caller may act on the block.
	CanOperate bool
}

// OperationRoster is the supervision roster of one block.
type OperationRoster struct {
	Instance            OperationRosterInstance `json:"instance"`
	Rows                []OperationRosterRow    `json:"rows"`
	PickupTimesLoaded   bool                    `json:"pickup_times_loaded"`
	PickupTimesRedacted bool                    `json:"pickup_times_redacted,omitempty"`
	// MovedFrom is set only on check-in responses that moved the child out
	// of another running session (#2386); empty means no name resolved.
	MovedFrom *string `json:"moved_from,omitempty"`
	// CanOperate reports whether the caller may act on this block (#3167).
	CanOperate bool `json:"can_operate"`
	// CanEditAttendance is separate from start/complete/reopen authority.
	CanEditAttendance bool `json:"can_edit_attendance"`
	// CanReportAbsence covers sick/excused block markers only.
	CanReportAbsence bool `json:"can_report_absence"`
}

// OperationRosterInstance is the block a roster belongs to.
type OperationRosterInstance struct {
	ID                  int64   `json:"id"`
	Title               string  `json:"title"`
	Status              string  `json:"status"`
	IsSpontaneous       bool    `json:"is_spontaneous"`
	ActiveGroupID       *int64  `json:"active_group_id,omitempty"`
	RoomID              int64   `json:"room_id"`
	RoomName            *string `json:"room_name,omitempty"`
	Date                string  `json:"date"`
	StartTime           string  `json:"start_time"`
	EndTime             string  `json:"end_time"`
	CanComplete         bool    `json:"can_complete"`
	CompleteAvailableAt string  `json:"complete_available_at"`
}

// OperationRosterRow is one child on a roster.
type OperationRosterRow struct {
	StudentID        int64                    `json:"student_id"`
	StudentName      string                   `json:"student_name"`
	SchoolClass      string                   `json:"school_class"`
	GroupName        string                   `json:"group_name"`
	Planned          bool                     `json:"planned"`
	IsUnplanned      bool                     `json:"is_unplanned"`
	CurrentlyPresent bool                     `json:"currently_present"`
	VisitID          *int64                   `json:"visit_id,omitempty"`
	Status           string                   `json:"status"`
	Substatus        *string                  `json:"substatus,omitempty"`
	Note             *string                  `json:"note,omitempty"`
	CheckedInAt      *string                  `json:"checked_in_at,omitempty"`
	CheckedOutAt     *string                  `json:"checked_out_at,omitempty"`
	VisitEntryTime   *string                  `json:"visit_entry_time,omitempty"`
	PickupTime       *string                  `json:"pickup_time"`
	Warnings         []OperationRosterWarning `json:"warnings,omitempty"`
	// ParallelPresentIn names the other running block where the child is
	// currently recorded present (#2265).
	ParallelPresentIn *OperationParallelPresence `json:"parallel_present_in,omitempty"`
	// CareDayStatus is the care-plan verdict for this child on the block's
	// date (#1747): "scheduled" | "not_scheduled" | "cancelled" | "unknown".
	// The rows stay in the payload so a child who turns up anyway can still
	// be checked in with one tap.
	CareDayStatus CareDayStatus `json:"care_day_status"`
}

// OperationParallelPresence identifies the other running block a roster
// row's parallel-presence hint points at (#2265).
type OperationParallelPresence struct {
	InstanceID int64  `json:"instance_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
}

// OperationRosterWarning flags a roster row (late arrival, class mismatch).
type OperationRosterWarning struct {
	Kind                  string  `json:"kind"`
	Message               string  `json:"message"`
	ExpectedArrival       *string `json:"expected_arrival,omitempty"`
	SlotStart             *string `json:"slot_start,omitempty"`
	ExpectedGroupID       *int64  `json:"expected_group_id,omitempty"`
	ExpectedGroupName     *string `json:"expected_group_name,omitempty"`
	CurrentEducationGroup *int64  `json:"current_education_group_id,omitempty"`
}

// StartedOperation is what a start or reopen answers.
type StartedOperation struct {
	InstanceID    int64
	Status        string
	ActiveGroupID int64
	Warnings      []InstanceConflictWarning
}

// SpontaneousStart is an ad-hoc block the caller creates and starts in one
// step. StartTime/EndTime carry the wall clock on 2000-01-01 UTC.
type SpontaneousStart struct {
	Date             calendar.Date
	StartTime        time.Time
	EndTime          time.Time
	Title            string
	Description      *string
	Notes            *string
	RoomID           int64
	ActivityGroupID  *int64
	StaffIDs         []int64
	CreatedByStaffID *int64
}

// SpontaneousCreateError wraps a failure of the create half of a
// spontaneous start, so the caller keeps the create-specific mapping.
type SpontaneousCreateError struct{ Err error }

func (e *SpontaneousCreateError) Error() string { return e.Err.Error() }
func (e *SpontaneousCreateError) Unwrap() error { return e.Err }

// ScheduledInstance is one block with the execution state of its Student
// Presence session, in the JSON shape the timetable routes answer with.
type ScheduledInstance struct {
	ID               int64         `json:"id"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	TenantID         int64         `json:"tenant_id"`
	Date             calendar.Date `json:"date"`
	ActivityGroupID  *int64        `json:"activity_group_id,omitempty"`
	CalendarPeriodID *int64        `json:"calendar_period_id,omitempty"`
	Title            string        `json:"title"`
	Description      *string       `json:"description,omitempty"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	RoomID           int64         `json:"room_id"`
	// RequiredStaff is the per-occurrence Personalbedarf pin (#1839).
	RequiredStaff    *int       `json:"required_staff,omitempty"`
	Status           string     `json:"status"`
	ActiveGroupID    *int64     `json:"active_group_id,omitempty"`
	ListKind         *string    `json:"list_kind,omitempty"`
	IsSpontaneous    bool       `json:"is_spontaneous"`
	UnderstaffedAck  bool       `json:"understaffed_ack"`
	UnderstaffedNote *string    `json:"understaffed_note,omitempty"`
	CancelReason     *string    `json:"cancel_reason,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	CreatedBy        *int64     `json:"created_by,omitempty"`
	StartedBy        *int64     `json:"started_by,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CompletedBy      *int64     `json:"completed_by,omitempty"`
	ReopenUntil      *time.Time `json:"reopen_until,omitempty"`
	// HasCompletionSnapshot reports a completion a reopen can restore.
	HasCompletionSnapshot bool `json:"-"`
}

// OperationQuery reads the operational day: the caller's planned blocks,
// the running sessions and the rosters.
type OperationQuery interface {
	PlannedNow(ctx context.Context, accountID int64, isAdmin bool, date calendar.Date, now time.Time, opts PlannedNowOptions) ([]OperationPlannedInstance, error)
	// ActiveSessions lists the day's running blocks with their plan windows,
	// keyed by live session (#2265).
	ActiveSessions(ctx context.Context, date calendar.Date) ([]OperationActiveSession, error)
	// SessionBlocks resolves, for the caller, the running blocks behind the
	// given live sessions of a day (#3281); supervisorStaffIDs maps each
	// session to its current supervisors.
	SessionBlocks(ctx context.Context, accountID int64, isAdmin bool, date calendar.Date, supervisorStaffIDs map[int64][]int64) ([]OperationSessionBlock, error)
	Roster(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*OperationRoster, error)
	RosterByActiveGroup(ctx context.Context, accountID int64, isAdmin bool, activeGroupID int64) (*OperationRoster, error)
	// EarliestPlannedBlockStartForClass returns the "HH:MM" start of the
	// first non-cancelled block of the date that addresses the school class,
	// "" when there is none (#2970).
	EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date calendar.Date) (string, error)
}

// OperationCommand is what staff do on a running day: start, complete and
// reopen blocks, check children in and out and patch their attendance.
type OperationCommand interface {
	Start(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartedOperation, error)
	// CreateAndStartSpontaneous creates a spontaneous block and starts it as
	// one atomic composition in the caller's request transaction.
	CreateAndStartSpontaneous(ctx context.Context, accountID int64, isAdmin bool, in SpontaneousStart) (*StartedOperation, error)
	Complete(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*ScheduledInstance, error)
	Reopen(ctx context.Context, accountID int64, isAdmin bool, instanceID int64) (*StartedOperation, error)
	CheckInStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error)
	CheckOutStudent(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64) (*OperationRoster, error)
	PatchAttendance(ctx context.Context, accountID int64, isAdmin bool, instanceID, studentID int64, patch AttendancePatch) (*OperationRosterRow, error)
}

// OperationCapability is the operational day of the Timetable owner.
type OperationCapability interface {
	OperationQuery
	OperationCommand
}
