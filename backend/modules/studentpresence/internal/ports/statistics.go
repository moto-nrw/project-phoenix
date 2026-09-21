package ports

import (
	"context"
	"time"
)

// The Statistik report (#2606) reads its own presence facts through the
// application service and every foreign fact through the ports below, which
// the composition root binds to the owning capabilities.

// StatisticsAccessEvent records the report scope, not the reported children's data.
type StatisticsAccessEvent struct {
	ActorAccountID int64
	ActorRole      string
	RangeStart     time.Time
	RangeEnd       time.Time
	AccessedAt     time.Time
	Metadata       map[string]string
}

// StatisticsAccessLog is the GDPR data-access log the report writes to.
type StatisticsAccessLog interface {
	RecordStatisticsAccess(context.Context, StatisticsAccessEvent) error
	SeenStatisticsAccessSince(context.Context, int64, map[string]string, time.Time) (bool, error)
}

// StatisticsRoom is the room metadata needed to price utilization.
type StatisticsRoom struct {
	ID       int64
	Name     string
	Capacity *int
}

type StatisticsRooms interface {
	StatisticsRooms(context.Context) ([]StatisticsRoom, error)
}

// StatisticsStudent carries only report identity and the owner's enrollment rule.
// The rule receives the report's single captured today, never a fresh clock read.
type StatisticsStudent struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
	GroupID     *int64
	GroupName   string
	EnrolledOn  func(day, today Date) bool
}

type StatisticsStudents interface {
	FindOverlappingWithGroups(ctx context.Context, from, to, today Date) ([]*StatisticsStudent, error)
}

type HolidayPeriod struct{ StartDate, EndDate Date }

type HolidayPeriods interface {
	StatisticsHolidayPeriods(context.Context, Date, Date) ([]HolidayPeriod, error)
}

type HolidayDates interface {
	HolidayDates(ctx context.Context, from, to Date) (map[Date]bool, error)
}

type ClosingDayDates interface {
	ClosingDayDates(ctx context.Context, from, to Date) (map[Date]bool, error)
}

type CourseInstance struct {
	CourseID           int64
	Name               string
	CategoryName       string
	MaxParticipants    int
	HeldInstances      int
	CancelledInstances int
}

type CourseParticipation struct {
	CourseID    int64
	StudentID   int64
	PresentDays int
	AbsentDays  int
	OpenDays    int
}

type CourseStatistics interface {
	CourseInstances(context.Context, Date, Date, Date) ([]CourseInstance, error)
	CourseParticipation(context.Context, Date, Date, Date) ([]CourseParticipation, error)
}

// StatusDay is a planned status (sick, excused, class trip) owned by Care Plan.
type StatusDay struct {
	StudentID int64
	Date      Date
	Status    string
}

type StatusDays interface {
	StatusDays(context.Context, Date, Date) ([]StatusDay, error)
}

// StatisticsRetention resolves the tenant retention windows the report names.
type StatisticsRetention interface {
	RoomRetentionDays(context.Context) (int, error)
	CourseRetentionDays(context.Context) (int, error)
}

// Own presence facts the report reads through the application service.

type AttendanceDays interface {
	ListAttendanceDays(ctx context.Context, from, to Date) ([]AttendanceDay, error)
}

type RoomUtilizations interface {
	RoomUtilization(context.Context, []StudentVisitWindow) ([]RoomUtilization, error)
}

type RetentionSettings interface {
	ListAcceptedRetentionSettings(context.Context) ([]StudentRetentionSetting, error)
}
