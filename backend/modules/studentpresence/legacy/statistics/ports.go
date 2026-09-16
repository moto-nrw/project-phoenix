package statistics

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// AccessEvent records the report scope, not the reported children's data.
type AccessEvent struct {
	ActorAccountID int64
	ActorRole      string
	RangeStart     time.Time
	RangeEnd       time.Time
	AccessedAt     time.Time
	Metadata       map[string]string
}

type accessLog interface {
	RecordStatisticsAccess(context.Context, AccessEvent) error
	SeenStatisticsAccessSince(context.Context, int64, map[string]string, time.Time) (bool, error)
}

// ReportRoom is the room metadata needed to price utilization.
type ReportRoom struct {
	ID       int64
	Name     string
	Capacity *int
}

// ReportStudent carries only report identity and the owner's enrollment rule.
// The rule receives the report's single captured today, never a fresh clock read.
type ReportStudent struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
	GroupID     *int64
	GroupName   string
	EnrolledOn  func(day, today timezone.Date) bool
}

type RetentionSetting struct {
	StudentID         int64
	DataRetentionDays int
}

type roomReader interface {
	StatisticsRooms(context.Context) ([]ReportRoom, error)
}

type HolidayPeriod struct{ StartDate, EndDate timezone.Date }

type calendarPeriods interface {
	StatisticsHolidayPeriods(context.Context, timezone.Date, timezone.Date) ([]HolidayPeriod, error)
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

type courseStatistics interface {
	CourseInstances(context.Context, timezone.Date, timezone.Date, timezone.Date) ([]CourseInstance, error)
	CourseParticipation(context.Context, timezone.Date, timezone.Date, timezone.Date) ([]CourseParticipation, error)
}

type AttendanceDay struct {
	StudentID int64
	Date      timezone.Date
}
type StatusDay struct {
	StudentID int64
	Date      timezone.Date
	Status    string
}
type attendanceDays interface {
	AttendanceDays(context.Context, timezone.Date, timezone.Date) ([]AttendanceDay, error)
}
type statusDays interface {
	StatusDays(context.Context, timezone.Date, timezone.Date) ([]StatusDay, error)
}
type StudentVisitWindow struct {
	StudentID int64     `json:"student_id"`
	StartAt   time.Time `json:"start_at"`
	EndAt     time.Time `json:"end_at"`
}
type RoomUtilization struct {
	RoomID           int64
	DaysUsed         int
	DistinctStudents int
	StudentMinutes   int
	PeakOccupancy    int
}

type roomUtilization interface {
	RoomUtilization(context.Context, []StudentVisitWindow) ([]RoomUtilization, error)
}
