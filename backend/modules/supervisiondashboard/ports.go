package supervisiondashboard

import (
	"context"
	"time"
)

// Date is a calendar day (YYYY-MM-DD) without clock or zone, the same shape
// the owner contracts the ports adapt use. Clocks travel as wall-clock
// strings (HH:MM). Every port reads the tenant in context; none of them
// writes.
type Date string

// Caller carries the request principal's projection-relevant rights. The
// access port resolves them once per request from the shared authorize rules.
type Caller struct {
	// AccountID is the authenticated account the planned-now query is
	// scoped to.
	AccountID int64
	// TokenAdmin is the token's admin flag the Timetable owner's planned-now
	// query keys its assignment filter on.
	TokenAdmin bool
	// AdminScope reports whether the principal holds the admin scope, which
	// grants session assignment regardless of supervision.
	AdminScope bool
	// OperationalOverview reports whether the caller sees every running
	// session of the school instead of only the supervised ones (#2380).
	OperationalOverview bool
	// CanReadSchedules gates the planned-now, active-session and capability
	// sections (schedules:read).
	CanReadSchedules bool
	// CanReadStudents gates the planning times (users:read).
	CanReadStudents bool
}

// Access resolves the caller's identity and rights for the current request.
type Access interface {
	// CurrentStaffID resolves the caller's staff record; nil when the
	// account is not linked to a person or staff row.
	CurrentStaffID(ctx context.Context) (*int64, error)
	Caller(ctx context.Context) (Caller, error)
	// FullStudentAccess reports whether the caller sees unredacted student
	// records (admin or verified staff, #2329). It is resolved only once a
	// session's visits are about to be projected.
	FullStudentAccess(ctx context.Context) (bool, error)
}

// Session is one running live session (active group) with its room facts.
type Session struct {
	ID int64
	// Name is the started activity's name; empty for room-only sessions.
	Name      string
	RoomID    *int64
	RoomName  string
	RoomColor *string
}

// SessionDirectory lists live sessions. Running and Supervised deliver their
// rows in German dictionary order (DIN 5007-1) of the room name, which the
// response order relies on.
type SessionDirectory interface {
	// Running lists every running session of the tenant.
	Running(ctx context.Context) ([]Session, error)
	// Supervised lists the sessions the caller currently supervises.
	Supervised(ctx context.Context) ([]Session, error)
	// SupervisedByStaff reports the running sessions the staff member
	// supervises, keyed by session id.
	SupervisedByStaff(ctx context.Context, staffID int64) (map[int64]struct{}, error)
	// Unclaimed lists the running sessions without a supervisor.
	Unclaimed(ctx context.Context) ([]UnclaimedGroup, error)
	// InRooms lists every running session in the supplied released rooms in
	// one bulk read.
	InRooms(ctx context.Context, roomIDs []int64) ([]RunningSession, error)
}

// Yard reads the Schulhof workflow state of a staff member.
type Yard interface {
	Status(ctx context.Context, staffID int64) (*SchulhofStatus, error)
}

// EducationGroups lists the caller's education groups with their room names.
type EducationGroups interface {
	MyGroups(ctx context.Context) ([]EducationalGroup, error)
}

// PlannedNowQuery scopes the Timetable owner's planned-now read.
type PlannedNowQuery struct {
	AccountID      int64
	TokenAdmin     bool
	Date           Date
	Now            time.Time
	HorizonMinutes int
	Limit          int
	IncludeRoster  bool
}

// Schedule reads the day's planned and running timetable instances.
type Schedule interface {
	PlannedNow(ctx context.Context, query PlannedNowQuery) ([]PlannedInstance, error)
	ActiveSessions(ctx context.Context, date Date) ([]ActiveSession, error)
}

// VisitRecord is one visit of a live session with the student's display
// facts. ExitTime is nil while the visit is open.
type VisitRecord struct {
	StudentID     int64
	ActiveGroupID int64
	EntryTime     time.Time
	ExitTime      *time.Time
	FirstName     string
	LastName      string
	SchoolClass   string
	GroupName     string
	Sick          bool
	SickSince     *time.Time
	Excused       bool
	ExcusedSince  *time.Time
	// PhotoPath is the stored path; the projection rewrites it to the
	// authenticated proxy URL.
	PhotoPath *string
}

// Attendance is today's attendance row of one student.
type Attendance struct {
	CheckInTime  *time.Time
	CheckOutTime *time.Time
}

// Presence reads live presence facts.
type Presence interface {
	// GroupVisits lists the visits of the session, open and closed.
	GroupVisits(ctx context.Context, activeGroupID int64) ([]VisitRecord, error)
	// AttendanceTimes returns today's attendance rows of the students.
	AttendanceTimes(ctx context.Context, studentIDs []int64) (map[int64]Attendance, error)
	TrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)
	OpenVisitsOfSessions(ctx context.Context, activeGroupIDs []int64) ([]VisitRecord, error)
}

// RoomDirectory answers which rooms the Facilities owner released. A release
// controls shared visibility; it never grants a supervision.
type RoomDirectory interface {
	Released(ctx context.Context) ([]ReleasedRoom, error)
}

type ReleasedRoom struct {
	ID   int64
	Name string
}

// RunningSession contains the facts one shared-room entry needs for a live
// session. An empty ActivityName is a room-only session, not a placeholder.
type RunningSession struct {
	ActiveGroupID      int64
	RoomID             int64
	ActivityName       string
	StartTime          time.Time
	SupervisorStaffIDs []int64
}

// Pickup is the effective pickup plan of one student for the day.
type Pickup struct {
	Date        Date
	WeekdayName string
	// PickupTime is the wall clock (HH:MM); nil marks a timeless exception.
	PickupTime  *string
	IsException bool
	Notes       string
	DayNotes    []DayNote
}

// Arrival is the effective arrival plan of one student for the day.
type Arrival struct {
	Date        Date
	WeekdayName string
	// ArrivalTime is the wall clock (HH:MM); nil marks a timeless exception.
	ArrivalTime *string
	IsException bool
	Notes       string
	DayNotes    []DayNote
}

// Planning reads the effective day plans of students.
type Planning interface {
	Pickups(ctx context.Context, studentIDs []int64, date Date) (map[int64]Pickup, error)
	Arrivals(ctx context.Context, studentIDs []int64, date Date) (map[int64]Arrival, error)
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
	// SpontaneousActivitiesEnabled reports whether staff may start
	// spontaneous activities from the web; false outside the open-rooms
	// care concept.
	SpontaneousActivitiesEnabled(ctx context.Context) (bool, error)
}

// Calendar supplies the tenant's calendar day and wall-clock rendering.
type Calendar interface {
	// DayOf is the calendar day (YYYY-MM-DD) of the instant.
	DayOf(at time.Time) Date
	// Weekday is the day of the week of a calendar day.
	Weekday(date Date) time.Weekday
	// Clock renders an instant as the local wall clock (HH:MM).
	Clock(at time.Time) string
}
