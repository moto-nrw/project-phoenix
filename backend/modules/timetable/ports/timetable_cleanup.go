package ports

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CleanupAuditSampleCap bounds the instance ids sampled into one child's
// audit metadata.
const CleanupAuditSampleCap = 10

// CleanupTimetable is the slice of the Timetable owner the retention reads
// and deletes through; dates are YYYY-MM-DD.
type CleanupTimetable interface {
	CountActivityInstances(ctx context.Context, before *string) (int, error)
	OldestActivityInstanceBefore(ctx context.Context, before *string) (*string, error)
	DeleteActivityInstancesBefore(ctx context.Context, before string) (int64, error)
	CountActivityExceptions(ctx context.Context, before *string) (int, error)
	OldestActivityExceptionBefore(ctx context.Context, before *string) (*string, error)
	DeleteActivityExceptionsBefore(ctx context.Context, before string) (int64, error)
	ListStudentInstanceRefsBefore(ctx context.Context, before string) ([]timetable.StudentInstanceRef, error)
}

// RetentionSettings resolves the tenant's timetable retention window in
// days (gdpr.timetable_retention_days) from the Settings Platform.
type RetentionSettings interface {
	TimetableRetentionDays(ctx context.Context) (int, error)
}

// StudentDeletionRecord is one audit.data_deletions row of a retention run:
// how many planned slots of one child the run removes.
type StudentDeletionRecord struct {
	TenantID          int64
	StudentID         int64
	RecordsDeleted    int
	RetentionDays     int
	CutoffDate        calendar.Date
	InstanceIDsSample []int64
}

// DeletionAudit appends the Audit Platform's data-deletion record.
type DeletionAudit interface {
	RecordTimetableRetention(ctx context.Context, record StudentDeletionRecord) error
}

// DeviationEventRetention deletes the Änderungsprotokoll rows whose
// occurrence date lies before the cutoff, in lockstep with the instances
// they annotate (#1886).
type DeviationEventRetention interface {
	DeleteDeviationEventsBefore(ctx context.Context, cutoff calendar.Date) (int64, error)
}

// TimetableCleanupDependencies wires the retention. Timetable and Audit are
// required, as are Settings. DeviationEvents skips the protocol step when nil.
// Clock defaults to the wall clock.
type TimetableCleanupDependencies struct {
	Timetable       CleanupTimetable
	Audit           DeletionAudit
	DeviationEvents DeviationEventRetention
	Settings        RetentionSettings
	Logger          *slog.Logger
	Clock           func() time.Time
}
