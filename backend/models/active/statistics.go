package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// AttendanceDayRow is one (student, calendar day) pair with at least one
// attendance record. Multiple check-ins on the same day collapse into one
// row — the statistics count days, not sessions (#2606).
type AttendanceDayRow struct {
	StudentID int64         `bun:"student_id"`
	Date      timezone.Date `bun:"date,type:date"`
}

// StatusDayRow is one uncleared status day (sick / excused / class_trip)
// inside the requested window.
type StatusDayRow struct {
	StudentID int64         `bun:"student_id"`
	Date      timezone.Date `bun:"date,type:date"`
	Status    string        `bun:"status"`
}

// RoomUtilizationRow aggregates every visit that overlapped the window,
// per room. Minutes are clamped to the window so a visit that started
// before `from` only contributes the part inside the report.
type RoomUtilizationRow struct {
	RoomID           int64 `bun:"room_id"`
	DaysUsed         int   `bun:"days_used"`
	DistinctStudents int   `bun:"distinct_students"`
	StudentMinutes   int   `bun:"student_minutes"`
	PeakOccupancy    int   `bun:"peak_occupancy"`
}

// StatisticsRepository serves the aggregate reads behind the Statistik
// page (#2606). All methods are tenant-scoped through the request context.
type StatisticsRepository interface {
	// AttendanceDays returns every distinct (student, day) with an
	// attendance record whose date lies in [from, to].
	AttendanceDays(ctx context.Context, from, to timezone.Date) ([]AttendanceDayRow, error)
	// StatusDays returns every uncleared status day in [from, to].
	StatusDays(ctx context.Context, from, to timezone.Date) ([]StatusDayRow, error)
	// RoomUtilization aggregates visits that overlapped [start, end) per room.
	// groupIDs restricts rows to students' current groups when non-empty.
	RoomUtilization(ctx context.Context, start, end time.Time, groupIDs []int64) ([]RoomUtilizationRow, error)
}
