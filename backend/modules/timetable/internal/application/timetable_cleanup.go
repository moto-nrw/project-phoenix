package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
//   - Retention resolves tenant override → registry default through Settings
//     Platform. Missing settings and lookup failures abort instead of guessing.

type timetableCleanup struct {
	deps   ports.TimetableCleanupDependencies
	logger *slog.Logger
	today  func() calendar.Date
}

// NewTimetableCleanup composes the Timetable owner's retention cleanup.
func NewTimetableCleanup(deps ports.TimetableCleanupDependencies) (timetable.TimetableCleanup, error) {
	if deps.Timetable == nil {
		return nil, errors.New("timetable cleanup: the timetable owner is required")
	}
	if deps.Settings == nil {
		return nil, errors.New("timetable cleanup: settings are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &timetableCleanup{deps: deps, logger: logger, today: calendar.CalendarDateClock(deps.Clock)}, nil
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
	retentionDays, err := s.resolveRetentionDays(ctx)
	if err != nil {
		return nil, err
	}
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
func (s *timetableCleanup) deleteExpired(ctx context.Context, cutoff calendar.Date) (*timetable.TimetableCleanupResult, error) {
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
	retentionDays, err := s.resolveRetentionDays(ctx)
	if err != nil {
		return nil, err
	}
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
	retentionDays, err := s.resolveRetentionDays(ctx)
	if err != nil {
		return nil, err
	}
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
func (s *timetableCleanup) oldestRows(ctx context.Context, before *string) (*calendar.Date, *calendar.Date, error) {
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

func optionalCleanupDate(value *string) (*calendar.Date, error) {
	if value == nil {
		return nil, nil
	}
	date, err := calendar.ParseDate(*value)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

// resolveRetentionDays rejects an unreadable or invalid retention window.
func (s *timetableCleanup) resolveRetentionDays(ctx context.Context) (int, error) {
	days, err := s.deps.Settings.TimetableRetentionDays(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve gdpr.timetable_retention_days: %w", err)
	}
	if days <= 0 {
		return 0, fmt.Errorf("gdpr.timetable_retention_days must be positive, got %d", days)
	}
	return days, nil
}

// collectStudentImpact counts, per child, the planned slots the run removes
// through the instance CASCADE, with a bounded sample of instance ids.
func (s *timetableCleanup) collectStudentImpact(ctx context.Context, cutoff calendar.Date) (cleanupImpact, error) {
	refs, err := s.deps.Timetable.ListStudentInstanceRefsBefore(ctx, cutoff.String())
	if err != nil {
		return cleanupImpact{}, err
	}
	impact := cleanupImpact{counts: map[int64]int{}, samples: map[int64][]int64{}}
	for _, ref := range refs {
		impact.counts[ref.StudentID]++
		if len(impact.samples[ref.StudentID]) < ports.CleanupAuditSampleCap {
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
	cutoff calendar.Date,
	impact cleanupImpact,
) error {
	if s.deps.Audit == nil {
		return fmt.Errorf("audit repo not configured")
	}
	for studentID, count := range impact.counts {
		record := ports.StudentDeletionRecord{
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
