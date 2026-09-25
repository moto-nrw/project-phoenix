package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableCleanupResult summarises one GDPR retention run (WP-B14).
type TimetableCleanupResult struct {
	Success                bool
	InstancesDeleted       int
	ExceptionsDeleted      int
	DeviationEventsDeleted int
	StudentsAffected       int
	RetentionDays          int
	CutoffDate             calendar.Date
	DurationMS             int64
}

// TimetableCleanupPreview counts what a run would delete, without deleting.
type TimetableCleanupPreview struct {
	InstancesToDelete  int
	ExceptionsToDelete int
	StudentsAffected   int
	RetentionDays      int
	CutoffDate         calendar.Date
	OldestInstance     *calendar.Date
	OldestException    *calendar.Date
}

// TimetableCleanupStats reports the tenant's timetable rows regardless of the
// retention window, for the CLI `stats` subcommand.
type TimetableCleanupStats struct {
	TotalInstances  int
	TotalExceptions int
	OldestInstance  *calendar.Date
	OldestException *calendar.Date
	RetentionDays   int
	CutoffDate      calendar.Date
}

// TimetableCleanup is the GDPR retention of the timetable (WP-B14): it
// deletes the activity instances (their staff and participant rows cascade),
// the activity exceptions and the Änderungsprotokoll entries older than the
// tenant's retention window, after one audit row per affected child. Every
// method runs in the tenant transaction the caller opened.
type TimetableCleanup interface {
	CleanupExpiredTimetableData(ctx context.Context) (*TimetableCleanupResult, error)
	PreviewExpiredTimetableData(ctx context.Context) (*TimetableCleanupPreview, error)
	GetStats(ctx context.Context) (*TimetableCleanupStats, error)
}
