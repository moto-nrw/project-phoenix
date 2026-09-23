package studentpresence

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// The Statistik report (#2606): attendance and absence quotas per child,
// group and period, room utilization and course participation over the same
// window. Everything is derived from data other modules already record —
// active.attendance, active.student_status_days, active.visits and the
// Betreuungsplan — nothing here writes business rows. The binding definitions
// live beside the computation in internal/application/statistics.

var (
	// ErrInvalidStatisticsRange is returned for from > to, a window ending in
	// the future, or a window longer than a school year plus a day.
	ErrInvalidStatisticsRange = errors.New("invalid statistics range")
	// ErrStatisticsAuditFailed wraps a failed data-access log write; the
	// report is withheld rather than served unlogged.
	ErrStatisticsAuditFailed = errors.New("statistics audit write failed")
)

// StatisticsSection names the part of the report a caller wants computed.
type StatisticsSection string

const (
	// StatisticsSectionAttendance is the child and group table.
	StatisticsSectionAttendance StatisticsSection = "attendance"
	// StatisticsSectionRooms is the room utilization table.
	StatisticsSectionRooms StatisticsSection = "rooms"
	// StatisticsSectionCourses is the course participation table (#2891).
	StatisticsSectionCourses StatisticsSection = "courses"
)

// StatisticsFilters selects the report window and an optional group restriction.
type StatisticsFilters struct {
	From     timezone.Date
	To       timezone.Date
	GroupIDs []int64
	// Sections limits what is computed. Empty means the whole report — the
	// screen loads every section at once so switching tabs costs no request.
	Sections []StatisticsSection
}

// Wants reports whether the section has to be computed.
func (f StatisticsFilters) Wants(section StatisticsSection) bool {
	if len(f.Sections) == 0 {
		return true
	}
	for _, s := range f.Sections {
		if s == section {
			return true
		}
	}
	return false
}

// StatisticsActor identifies who requests the report for the GDPR access log.
type StatisticsActor struct {
	AccountID int64
	Role      string
}

// StatisticsStudentRow is one child in the report.
type StatisticsStudentRow struct {
	StudentID       int64
	FirstName       string
	LastName        string
	SchoolClass     string
	GroupID         *int64
	GroupName       string
	PresentDays     int
	SickDays        int
	ExcusedDays     int
	UnexplainedDays int
	CareDays        int
	// AttendanceRate is present/care in percent; nil when there are no
	// care days in the window.
	AttendanceRate *float64
}

// StatisticsGroupRow aggregates the children of one education group. GroupID
// 0 is the pseudo group for children without a group.
type StatisticsGroupRow struct {
	GroupID         int64
	Name            string
	StudentCount    int
	PresentDays     int
	SickDays        int
	ExcusedDays     int
	UnexplainedDays int
	AttendanceRate  *float64
}

// StatisticsRoomRow is the utilization of one room over the window.
type StatisticsRoomRow struct {
	RoomID           int64
	Name             string
	Capacity         *int
	DaysUsed         int
	DistinctStudents int
	StudentMinutes   int
	PeakOccupancy    int
	// PeakUtilizationPercent is peak / capacity; nil without a capacity.
	PeakUtilizationPercent *float64
}

// StatisticsExcludedDays explains why weekdays were removed from the care-day
// count. The buckets may overlap (a closing day inside the holidays); Total is
// the size of the union.
type StatisticsExcludedDays struct {
	Total          int
	PublicHolidays int
	ClosingDays    int
	HolidayPeriods int
}

// StatisticsCourseRow is one course over the window.
type StatisticsCourseRow struct {
	CourseID     int64
	Name         string
	CategoryName string
	// MaxParticipants is the Teilnehmergrenze; 0 means unlimited.
	MaxParticipants    int
	HeldInstances      int
	CancelledInstances int
	// StudentCount is how many children had at least one decided or open
	// attendance row in the window.
	StudentCount int
	PresentDays  int
	AbsentDays   int
	OpenDays     int
	// ParticipationRate is present / (present + absent) in percent; nil when
	// nothing was decided yet.
	ParticipationRate *float64
	// OccupancyPercent is StudentCount against the Teilnehmergrenze; nil
	// without a limit.
	OccupancyPercent *float64
}

// StatisticsCourseStudentRow is one child in one course.
type StatisticsCourseStudentRow struct {
	StudentID         int64
	FirstName         string
	LastName          string
	SchoolClass       string
	GroupName         string
	CourseID          int64
	CourseName        string
	PresentDays       int
	AbsentDays        int
	OpenDays          int
	ParticipationRate *float64
}

// StatisticsReport is the full statistics result.
type StatisticsReport struct {
	From         timezone.Date
	To           timezone.Date
	CareDays     int
	ExcludedDays StatisticsExcludedDays
	Students     []StatisticsStudentRow
	Groups       []StatisticsGroupRow
	Rooms        []StatisticsRoomRow
	// RoomDataDays is the longest visit-retention window among the children
	// this report covers — the group filter narrows it with the population.
	// Individual children may have shorter windows.
	RoomDataDays int
	// RoomDataFrom is the earliest date room data can still exist for.
	RoomDataFrom timezone.Date
	Totals       StatisticsGroupRow
	// Courses is one row per course, CourseStudents one row per (child,
	// course) — the two views of the participation section (#2891).
	Courses        []StatisticsCourseRow
	CourseStudents []StatisticsCourseStudentRow
	CourseTotals   StatisticsCourseRow
	// CourseDataDays is the tenant's Betreuungsplan retention window; finished
	// occurrences older than that are deleted, so the section cannot reach
	// behind CourseDataFrom.
	CourseDataDays int
	CourseDataFrom timezone.Date
}

// StatisticsReports is the statistics use case.
type StatisticsReports interface {
	// Report computes the statistics for the window. It records one
	// data-access log row per actor and window (deduplicated for 15 min).
	Report(ctx context.Context, filters StatisticsFilters, actor StatisticsActor) (*StatisticsReport, error)
	// ReportForExport computes the report and records an export access
	// row every time (no deduplication — every download is evidence).
	ReportForExport(ctx context.Context, filters StatisticsFilters, actor StatisticsActor, format string) (*StatisticsReport, error)
}
