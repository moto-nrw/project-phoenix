package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// GDPR retention of the timetable tables (WP-B14, #3424 slice S5): deletes
// schedule.activity_instances (and through CASCADE schedule.instance_staff
// and schedule.instance_students), schedule.activity_exceptions and the
// Änderungsprotokoll rows older than the tenant's retention window.
//
//   - Two independent deletes per run: instances age by date, exceptions by
//     exception_date. Every instance status past retention goes; the
//     materializer never writes past-dated rows, so a planned row that old is
//     orphaned data.
//   - The caller supplies the tenant transaction; every read and delete runs
//     inside it and RLS enforces the tenant boundary.
//   - One audit.data_deletions row per affected child, written BEFORE the
//     deletes, so an audit failure rolls the whole run back. Exceptions carry
//     no personal data and are only logged.
//   - Retention days resolve tenant override → registry default through the
//     Settings Platform; 365 is the last resort when no settings are wired.

// cleanupAuditSampleCap bounds the instance ids sampled into one child's
// audit metadata.
const cleanupAuditSampleCap = 10

// timetableRetentionDefaultDays is the last-resort window. Production never
// reaches it: the settings registry defines 365 as the default.
const timetableRetentionDefaultDays = 365

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
	CutoffDate        timezone.Date
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
	DeleteDeviationEventsBefore(ctx context.Context, cutoff timezone.Date) (int64, error)
}

// TimetableCleanupDependencies wires the retention. Timetable and Audit are
// required; Settings falls back to 365 days and DeviationEvents skips the
// protocol step when nil. Clock defaults to the wall clock.
type TimetableCleanupDependencies struct {
	Timetable       CleanupTimetable
	Audit           DeletionAudit
	DeviationEvents DeviationEventRetention
	Settings        RetentionSettings
	Logger          *slog.Logger
	Clock           func() time.Time
}

type timetableCleanup struct {
	deps   TimetableCleanupDependencies
	logger *slog.Logger
	today  func() timezone.Date
}

// NewTimetableCleanup composes the Timetable owner's retention cleanup.
func NewTimetableCleanup(deps TimetableCleanupDependencies) (timetable.TimetableCleanup, error) {
	if deps.Timetable == nil {
		return nil, errors.New("timetable cleanup: the timetable owner is required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &timetableCleanup{deps: deps, logger: logger, today: timezone.CalendarDateClock(deps.Clock)}, nil
}

// cleanupImpact is the per-child count of planned slots a run removes, with
// a bounded sample of their instance ids.
type cleanupImpact struct {
	counts  map[int64]int
	samples map[int64][]int64
}

func (s *timetableCleanup) CleanupExpiredTimetableData(ctx context.Context) (*timetable.TimetableCleanupResult, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("timetable cleanup: no tenant in context")
	}
	start := time.Now()
	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := s.today().AddDays(-retentionDays)

	impact, err := s.collectStudentImpact(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect student impact: %w", err)
	}
	if err := s.writeStudentAuditRows(ctx, tenantID, retentionDays, cutoff, impact); err != nil {
		return nil, fmt.Errorf("write audit rows: %w", err)
	}
	result, err := s.deleteExpired(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	result.Success = true
	result.StudentsAffected = len(impact.counts)
	result.RetentionDays = retentionDays
	result.CutoffDate = cutoff
	result.DurationMS = time.Since(start).Milliseconds()

	s.logger.Info("timetable cleanup completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int("instances_deleted", result.InstancesDeleted),
		slog.Int("exceptions_deleted", result.ExceptionsDeleted),
		slog.Int("deviation_events_deleted", result.DeviationEventsDeleted),
		slog.Int("students_affected", result.StudentsAffected),
		slog.Int("retention_days", retentionDays),
		slog.String("cutoff_date", cutoff.Format("2006-01-02")),
		slog.Int64("duration_ms", result.DurationMS),
	)
	return result, nil
}

// deleteExpired removes the instances (CASCADE takes their staff and
// participant rows), the exceptions and, keyed on the same occurrence date,
// the Änderungsprotokoll rows of those slots.
func (s *timetableCleanup) deleteExpired(ctx context.Context, cutoff timezone.Date) (*timetable.TimetableCleanupResult, error) {
	instancesDeleted, err := s.deps.Timetable.DeleteActivityInstancesBefore(ctx, cutoff.String())
	if err != nil {
		return nil, fmt.Errorf("delete activity_instances: %w", err)
	}
	exceptionsDeleted, err := s.deps.Timetable.DeleteActivityExceptionsBefore(ctx, cutoff.String())
	if err != nil {
		return nil, fmt.Errorf("delete activity_exceptions: %w", err)
	}
	var deviationEventsDeleted int64
	if s.deps.DeviationEvents != nil {
		deviationEventsDeleted, err = s.deps.DeviationEvents.DeleteDeviationEventsBefore(ctx, cutoff)
		if err != nil {
			return nil, fmt.Errorf("delete deviation_events: %w", err)
		}
	}
	return &timetable.TimetableCleanupResult{
		InstancesDeleted:       int(instancesDeleted),
		ExceptionsDeleted:      int(exceptionsDeleted),
		DeviationEventsDeleted: int(deviationEventsDeleted),
	}, nil
}

func (s *timetableCleanup) PreviewExpiredTimetableData(ctx context.Context) (*timetable.TimetableCleanupPreview, error) {
	if tenant.FromContext(ctx) == 0 {
		return nil, fmt.Errorf("timetable cleanup preview: no tenant in context")
	}
	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := s.today().AddDays(-retentionDays)
	before := cutoff.String()

	instancesToDelete, err := s.deps.Timetable.CountActivityInstances(ctx, &before)
	if err != nil {
		return nil, fmt.Errorf("count activity_instances: %w", err)
	}
	exceptionsToDelete, err := s.deps.Timetable.CountActivityExceptions(ctx, &before)
	if err != nil {
		return nil, fmt.Errorf("count activity_exceptions: %w", err)
	}
	impact, err := s.collectStudentImpact(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect student impact: %w", err)
	}
	oldestInstance, oldestException, err := s.oldestRows(ctx, &before)
	if err != nil {
		return nil, err
	}
	return &timetable.TimetableCleanupPreview{
		InstancesToDelete:  instancesToDelete,
		ExceptionsToDelete: exceptionsToDelete,
		StudentsAffected:   len(impact.counts),
		RetentionDays:      retentionDays,
		CutoffDate:         cutoff,
		OldestInstance:     oldestInstance,
		OldestException:    oldestException,
	}, nil
}

func (s *timetableCleanup) GetStats(ctx context.Context) (*timetable.TimetableCleanupStats, error) {
	if tenant.FromContext(ctx) == 0 {
		return nil, fmt.Errorf("timetable cleanup stats: no tenant in context")
	}
	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := s.today().AddDays(-retentionDays)

	totalInstances, err := s.deps.Timetable.CountActivityInstances(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("count activity_instances: %w", err)
	}
	totalExceptions, err := s.deps.Timetable.CountActivityExceptions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("count activity_exceptions: %w", err)
	}
	oldestInstance, oldestException, err := s.oldestRows(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &timetable.TimetableCleanupStats{
		TotalInstances:  totalInstances,
		TotalExceptions: totalExceptions,
		OldestInstance:  oldestInstance,
		OldestException: oldestException,
		RetentionDays:   retentionDays,
		CutoffDate:      cutoff,
	}, nil
}

// oldestRows reads the oldest instance and exception date, optionally only
// among the rows before the cutoff.
func (s *timetableCleanup) oldestRows(ctx context.Context, before *string) (*timezone.Date, *timezone.Date, error) {
	oldestInstance, err := s.deps.Timetable.OldestActivityInstanceBefore(ctx, before)
	if err != nil {
		return nil, nil, fmt.Errorf("oldest activity_instance: %w", err)
	}
	oldestException, err := s.deps.Timetable.OldestActivityExceptionBefore(ctx, before)
	if err != nil {
		return nil, nil, fmt.Errorf("oldest activity_exception: %w", err)
	}
	instance, err := optionalCleanupDate(oldestInstance)
	if err != nil {
		return nil, nil, fmt.Errorf("oldest activity_instance: %w", err)
	}
	exception, err := optionalCleanupDate(oldestException)
	if err != nil {
		return nil, nil, fmt.Errorf("oldest activity_exception: %w", err)
	}
	return instance, exception, nil
}

func optionalCleanupDate(value *string) (*timezone.Date, error) {
	if value == nil {
		return nil, nil
	}
	date, err := timezone.ParseDate(*value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// resolveRetentionDays uses the tenant's window; a failed or non-positive
// resolution keeps the cleanup on the 365-day default instead of failing.
func (s *timetableCleanup) resolveRetentionDays(ctx context.Context) int {
	if s.deps.Settings == nil {
		return timetableRetentionDefaultDays
	}
	if days, err := s.deps.Settings.TimetableRetentionDays(ctx); err == nil && days > 0 {
		return days
	}
	return timetableRetentionDefaultDays
}

// collectStudentImpact counts, per child, the planned slots the run removes
// through the instance CASCADE, with a bounded sample of instance ids.
func (s *timetableCleanup) collectStudentImpact(ctx context.Context, cutoff timezone.Date) (cleanupImpact, error) {
	refs, err := s.deps.Timetable.ListStudentInstanceRefsBefore(ctx, cutoff.String())
	if err != nil {
		return cleanupImpact{}, err
	}
	impact := cleanupImpact{counts: map[int64]int{}, samples: map[int64][]int64{}}
	for _, ref := range refs {
		impact.counts[ref.StudentID]++
		if len(impact.samples[ref.StudentID]) < cleanupAuditSampleCap {
			impact.samples[ref.StudentID] = append(impact.samples[ref.StudentID], ref.InstanceID)
		}
	}
	return impact, nil
}

// writeStudentAuditRows appends one data-deletion record per affected child
// before anything is deleted, so a failure rolls the run back.
func (s *timetableCleanup) writeStudentAuditRows(
	ctx context.Context,
	tenantID int64,
	retentionDays int,
	cutoff timezone.Date,
	impact cleanupImpact,
) error {
	if s.deps.Audit == nil {
		return fmt.Errorf("audit repo not configured")
	}
	for studentID, count := range impact.counts {
		record := StudentDeletionRecord{
			TenantID:          tenantID,
			StudentID:         studentID,
			RecordsDeleted:    count,
			RetentionDays:     retentionDays,
			CutoffDate:        cutoff,
			InstanceIDsSample: impact.samples[studentID],
		}
		if err := s.deps.Audit.RecordTimetableRetention(ctx, record); err != nil {
			return fmt.Errorf("audit row for student %d: %w", studentID, err)
		}
	}
	return nil
}
