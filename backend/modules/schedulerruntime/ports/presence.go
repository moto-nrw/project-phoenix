package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// DailySessionsEnded is what ending a school's daily sessions reports.
// EndedActiveGroupIDs are the active groups the session end closed; the
// Worker completes their timetable instances. Success and Errors report
// single failures, which do not fail the session end.
type DailySessionsEnded struct {
	SessionsEnded       int
	VisitsEnded         int
	SupervisorsEnded    int
	EndedActiveGroupIDs []int64
	Success             bool
	Errors              []string
}

// SessionMaintenance ends a school's daily sessions and sweeps abandoned
// ones (Student Presence).
type SessionMaintenance interface {
	EndDailySessions(ctx context.Context) (*DailySessionsEnded, error)
	CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error)
}

// ExpiredVisitCleanup reports the retention sweep of a school's visits.
type ExpiredVisitCleanup struct {
	StudentsProcessed int
	RecordsDeleted    int64
	Success           bool
}

// StaleSupervisorCleanup reports the supervisor records from previous days
// that were closed.
type StaleSupervisorCleanup struct {
	RecordsClosed int
	Success       bool
}

// StaleAttendanceCleanup reports the attendance records from previous days
// that were closed. Success is false when some of them failed; Errors names
// those failures.
type StaleAttendanceCleanup struct {
	RecordsClosed    int
	StudentsAffected int
	Success          bool
	Errors           []string
}

// PresenceCleanup is the nightly cleanup of a school's presence records
// (Student Presence).
type PresenceCleanup interface {
	CleanupExpiredVisits(ctx context.Context) (*ExpiredVisitCleanup, error)
	CleanupStaleSupervisors(ctx context.Context) (*StaleSupervisorCleanup, error)
	CleanupStaleAttendance(ctx context.Context) (*StaleAttendanceCleanup, error)
}

// StudentStatusFlagArchiver archives a school's legacy end-of-day status flag
// into the students' status days for date and clears the flag (Care Plan).
// Care Plan decides the archived status and its source. Column names must be
// trusted constants, never user input.
type StudentStatusFlagArchiver interface {
	ArchiveEndOfDayStatusFlag(ctx context.Context, flagColumn, sinceColumn string, date calendar.Date, reportedFallback time.Time) (int64, error)
}

// PWAUsageCleanupResult reports the retention sweep of a school's standalone
// usage rows.
type PWAUsageCleanupResult struct {
	RowsDeleted   int
	RetentionDays int
}

// PWAUsageCleanup sweeps a school's standalone usage rows past their
// retention (Delivery Platform).
type PWAUsageCleanup func(ctx context.Context) (PWAUsageCleanupResult, error)

// InstanceOverdueBroadcaster announces a planned timetable block whose start
// lies past the school's overdue threshold to the whole school, and asks its
// live-supervision views to refetch (Delivery Platform's live events).
// startTime is the wall clock as HH:MM:SS.
type InstanceOverdueBroadcaster interface {
	BroadcastInstanceOverdue(tenantID, instanceID int64, date, startTime string, roomID int64) error
	BroadcastInstanceOverdueRefresh(tenantID, instanceID int64) error
}
