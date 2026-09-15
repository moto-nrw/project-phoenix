package grouplive

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

// PendingExcusedReader is the slice of the Care Plan excused-request
// contract the planning badge reads its note from.
type PendingExcusedReader interface {
	// PendingByStudentForDate returns, per student, the newest pending
	// request covering the day, scoped to the caller's review reach.
	PendingByStudentForDate(ctx context.Context, date excusedrequests.Date) (map[int64]*excusedrequests.Request, error)
}

// Date is a calendar day (YYYY-MM-DD) without clock or zone, the same shape
// the owner contracts the ports adapt use. Clocks travel as wall-clock
// strings (HH:MM). Every port reads the tenant in context; none of them
// writes.
type Date string

// Caller carries the request principal's projection-relevant rights. The
// access port resolves them once per request from the shared authorize rules.
type Caller struct {
	// CanReadGroups is the groups:read permission gating the group-derived
	// sections (room status, transfers, tracking indicators).
	CanReadGroups bool
	// CanReviewExcusedRequests mirrors authorize.CanReviewExcusedAbsenceRequests.
	CanReviewExcusedRequests bool
	// OperationalOverview reports whether the caller may see every tenant
	// group instead of only the supervised ones.
	OperationalOverview bool
}

// Access resolves the caller's rights for the current request.
type Access interface {
	Caller(ctx context.Context) (Caller, error)
	// FullStudentAccess reports whether the caller sees unredacted student
	// records (admin or verified staff, #2329). It is resolved only once a
	// roster is about to be projected.
	FullStudentAccess(ctx context.Context) (bool, error)
}

// GroupRecord is one education group as the projection needs it.
type GroupRecord struct {
	ID     int64
	Name   string
	RoomID *int64
}

// GroupDirectory lists groups. SupervisedGroups and TenantGroups deliver
// their rows in German dictionary order (DIN 5007-1), which selectGroup and
// the response order rely on.
type GroupDirectory interface {
	// SupervisedGroups are the caller's own groups, including substitutions.
	SupervisedGroups(ctx context.Context) ([]GroupRecord, error)
	// TenantGroups are every group of the tenant.
	TenantGroups(ctx context.Context) ([]GroupRecord, error)
	// SubstitutedGroupIDs marks the groups the caller reaches only through a
	// substitution.
	SubstitutedGroupIDs(ctx context.Context) (map[int64]bool, error)
	// GroupRoomNames resolves the room name of every group that has a room.
	GroupRoomNames(ctx context.Context, groupIDs []int64) (map[int64]string, error)
}

// RosterStudent is a group member with the directory facts the roster shows.
type RosterStudent struct {
	ID           int64
	FirstName    string
	LastName     string
	SchoolClass  string
	Sick         bool
	SickSince    *time.Time
	Excused      bool
	ExcusedSince *time.Time
	// PhotoPath is the stored path; the projection rewrites it to the
	// authenticated proxy URL.
	PhotoPath *string
}

// Roster lists a group's members and their care participation.
type Roster interface {
	// GroupMembers returns the participation candidates of the group with
	// their person names; members without a person row are omitted.
	GroupMembers(ctx context.Context, groupID int64) ([]RosterStudent, error)
	// CareParticipants reports which of the students take part in care on
	// the date. The resolver loads open roster check-ins itself.
	CareParticipants(ctx context.Context, studentIDs []int64, date Date) (map[int64]bool, error)
}

// Location is a student's resolved live location.
type Location struct {
	Name      string
	Since     *time.Time
	RoomColor *string
}

// Attendance is today's attendance row of one student.
type Attendance struct {
	// Recorded reports whether an attendance row exists for the day.
	Recorded bool
	// Present reports whether the child is still on the premises.
	Present      bool
	CheckInTime  *time.Time
	CheckOutTime *time.Time
}

// EffectiveStatus is the resolved status-day precedence: sick wins over
// class trip wins over excused.
type EffectiveStatus struct {
	Sick           bool
	ClassTrip      bool
	Excused        bool
	SickSince      *time.Time
	ClassTripSince *time.Time
	ExcusedSince   *time.Time
}

// PresenceSnapshot answers location questions for the students it was
// loaded for without further queries.
type PresenceSnapshot interface {
	// Location resolves the display location under the caller's access.
	Location(studentID int64, fullAccess bool) Location
	// Attendance returns the day's attendance row; ok is false without one.
	Attendance(studentID int64) (attendance Attendance, ok bool)
	// CurrentRoomID is the room of the student's open visit, if any.
	CurrentRoomID(studentID int64) *int64
}

// Presence reads live presence and scheduled status facts.
type Presence interface {
	Snapshot(ctx context.Context, studentIDs []int64, date Date) (PresenceSnapshot, error)
	EffectiveStatuses(ctx context.Context, studentIDs []int64, date Date) (map[int64]EffectiveStatus, error)
	TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)
}

// Arrival is the effective arrival plan of one student for the day.
type Arrival struct {
	// ArrivalTime is a wall-clock value; nil marks a timeless exception.
	ArrivalTime *time.Time
	IsException bool
	Notes       string
	DayNotes    []string
}

// Pickup is the effective pickup plan of one student for the day.
type Pickup struct {
	Date        Date
	WeekdayName string
	// PickupTime is a wall-clock value; nil marks a timeless exception.
	PickupTime  *time.Time
	IsException bool
	Notes       string
	DayNotes    []DayNote
}

// Day-planning reason codes are part of the students API wire contract
// (day_planning_reason); the owner's rules produce them byte-identically.
const (
	DayReasonSick             = "sick"
	DayReasonExcused          = "excused"
	DayReasonClassTrip        = "class_trip"
	DayReasonArrivalException = "arrival_exception"
	DayReasonPickupException  = "pickup_exception"
	DayReasonArrivalSchedule  = "arrival_schedule"
	DayReasonPickupSchedule   = "pickup_schedule"
	DayReasonTimetable        = "timetable"
	DayReasonUnplanned        = "unplanned_attendance"
	DayReasonNoPlan           = "no_plan"
)

// DayInputs are the resolved facts the day-planning precedence decides from.
type DayInputs struct {
	Present      bool
	Sick         bool
	ClassTrip    bool
	Excused      bool
	Arrival      *Arrival
	Pickup       *Pickup
	HasTimetable bool
}

// DayDecision is the outcome of the day-planning precedence rules.
type DayDecision struct {
	ComesToday     bool
	Reason         string
	ExceptionNotes string
}

// Planning reads the day plans and applies the owner's precedence rules.
type Planning interface {
	Arrivals(ctx context.Context, studentIDs []int64, date Date) (map[int64]Arrival, error)
	Pickups(ctx context.Context, studentIDs []int64, date Date) (map[int64]Pickup, error)
	// TimetablePlannedStudentIDs are the students with a planned timetable
	// instance on the date whose care day still expects them.
	TimetablePlannedStudentIDs(ctx context.Context, studentIDs []int64, date Date) (map[int64]bool, error)
	// DecideDay applies the shared day-planning precedence.
	DecideDay(inputs DayInputs) DayDecision
}

// TransferReader lists the group handovers active on a date.
type TransferReader interface {
	GroupHandovers(ctx context.Context, groupID int64, date Date) ([]Transfer, error)
}

// Settings reads the tenant settings the projection depends on.
type Settings interface {
	// Prepare prefetches the request's settings snapshot and returns the
	// context to continue with.
	Prepare(ctx context.Context) (context.Context, error)
	StudentPhotosEnabled(ctx context.Context) (bool, error)
	// TrackingIndicatorLabels returns the configured labels, empty when the
	// indicators are disabled.
	TrackingIndicatorLabels(ctx context.Context) ([]string, error)
}

// Calendar supplies the tenant's calendar day and wall-clock rendering.
type Calendar interface {
	// Today is the current calendar day (YYYY-MM-DD).
	Today() Date
	// Clock renders an instant as the local wall clock (HH:MM).
	Clock(at time.Time) string
}
