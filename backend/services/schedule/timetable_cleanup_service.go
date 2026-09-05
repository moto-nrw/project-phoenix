// Package schedule — timetable cleanup service (WP-B14).
//
// GDPR retention cleanup for the timetable tables: deletes
// schedule.activity_instances (and via CASCADE, schedule.instance_staff +
// schedule.instance_students) and schedule.activity_exceptions rows older
// than the tenant-configured retention window.
//
// Design notes:
//
//   - Two separate deletes per tenant run. activity_instances and
//     activity_exceptions are independent tables with different "age"
//     predicates (date vs exception_date). CASCADE handles the instance_*
//     children automatically.
//
//   - All statuses past retention are deleted (planned / active / completed /
//     cancelled). Materialization never produces past-dated rows, so any
//     planned row older than retention is orphaned data.
//
//   - Caller supplies tenant context. The service trusts ctx has an active
//     WithTenantTx — both deletes run inside the caller's transaction and
//     RLS enforces the tenant boundary. The repositories add `tenant_id = ?`
//     predicates as defense-in-depth.
//
//   - Per-student audit rows (one row per affected student, not per run).
//     activity_exceptions carry no PII (template-scoped) and are not logged
//     to audit.data_deletions — slog-only. The audit writes land BEFORE the
//     deletes; a failure anywhere rolls back everything atomically.
//
//   - Retention resolution: tenant DB override → registry default. The 365
//     literal in the service is a last-resort fallback only reached when the
//     settings service is not wired (tests); in production the registry
//     definition in services/config/defaults/timetable.go supplies the default.
package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// instanceDateColumn / exceptionDateColumn are the retention-age columns the
// cleanup predicates run on.
const (
	instanceDateColumn  = "date"
	exceptionDateColumn = "exception_date"
)

// auditInstanceIDSampleCap bounds the number of instance IDs sampled into a
// per-student audit row's metadata. Keeps the metadata JSONB small while still
// giving compliance a handful of row identifiers for forensic lookup.
const auditInstanceIDSampleCap = 10

// timetableRetentionDefaultDays is a last-resort fallback. In production this
// is unreachable because the registry definition in
// services/config/defaults/timetable.go registers 365 at init time. Kept so
// the service is callable in tests without settings wiring.
const timetableRetentionDefaultDays = 365

// TimetableCleanupResult summarises one cleanup run.
type TimetableCleanupResult struct {
	Success                bool
	InstancesDeleted       int
	ExceptionsDeleted      int
	DeviationEventsDeleted int
	StudentsAffected       int
	RetentionDays          int
	CutoffDate             timezone.Date
	DurationMS             int64
}

// TimetableCleanupPreview is returned by PreviewExpiredTimetableData —
// identical shape to Result except nothing was deleted.
type TimetableCleanupPreview struct {
	InstancesToDelete  int
	ExceptionsToDelete int
	StudentsAffected   int
	RetentionDays      int
	CutoffDate         timezone.Date
	OldestInstance     *timezone.Date
	OldestException    *timezone.Date
}

// TimetableCleanupStats returns row counts and oldest timestamps across the
// tenant's timetable data for the CLI `stats` subcommand.
type TimetableCleanupStats struct {
	TotalInstances  int
	TotalExceptions int
	OldestInstance  *timezone.Date
	OldestException *timezone.Date
	RetentionDays   int
	CutoffDate      timezone.Date
}

// TimetableCleanupService drives GDPR retention cleanup for the timetable
// tables. All methods assume tenant context has been established by the
// caller (WithTenantTx).
type TimetableCleanupService interface {
	CleanupExpiredTimetableData(ctx context.Context) (*TimetableCleanupResult, error)
	PreviewExpiredTimetableData(ctx context.Context) (*TimetableCleanupPreview, error)
	GetStats(ctx context.Context) (*TimetableCleanupStats, error)
}

// timetableCleanupService implements TimetableCleanupService.
type timetableCleanupService struct {
	instanceRepo        scheduleModel.ActivityInstanceRepository
	exceptionRepo       scheduleModel.ActivityExceptionRepository
	instanceStudentRepo scheduleModel.InstanceStudentRepository
	auditRepo           audit.DataDeletionRepository
	deviationEventRepo  audit.DeviationEventRepository
	settings            config.SettingsService
	logger              *slog.Logger
	today               func() timezone.Date
}

// NewTimetableCleanupService constructs the cleanup service. settings may be
// nil in tests; when nil, retention resolves to the last-resort default 365.
// deviationEventRepo may be nil in legacy tests; when nil, the protocol
// cleanup step is skipped.
func NewTimetableCleanupService(
	instanceRepo scheduleModel.ActivityInstanceRepository,
	exceptionRepo scheduleModel.ActivityExceptionRepository,
	instanceStudentRepo scheduleModel.InstanceStudentRepository,
	auditRepo audit.DataDeletionRepository,
	deviationEventRepo audit.DeviationEventRepository,
	settings config.SettingsService,
	logger *slog.Logger,
	clocks ...func() time.Time,
) TimetableCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &timetableCleanupService{
		instanceRepo:        instanceRepo,
		exceptionRepo:       exceptionRepo,
		instanceStudentRepo: instanceStudentRepo,
		auditRepo:           auditRepo,
		deviationEventRepo:  deviationEventRepo,
		settings:            settings,
		logger:              logger,
		today:               timezone.CalendarDateClock(clocks...),
	}
}

// CleanupExpiredTimetableData deletes activity_instances + activity_exceptions
// older than the resolved retention window for the tenant in ctx. Writes one
// audit.data_deletions row per affected student before the deletes.
func (s *timetableCleanupService) CleanupExpiredTimetableData(ctx context.Context) (*TimetableCleanupResult, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("timetable cleanup: no tenant in context")
	}

	start := time.Now()
	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := s.cutoffFor(retentionDays)

	// 1. Collect per-student impact for the audit log.
	studentCounts, sampleIDs, err := s.collectStudentImpact(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect student impact: %w", err)
	}

	// 2. Write one audit row per affected student BEFORE the deletes. Inside
	//    the caller's WithTenantTx, so an audit failure rolls back the
	//    deletes. No audit rows for exceptions (template-scoped, no PII).
	if err := s.writeStudentAuditRows(ctx, tenantID, retentionDays, cutoff, studentCounts, sampleIDs); err != nil {
		return nil, fmt.Errorf("write audit rows: %w", err)
	}

	// 3. Delete activity_instances. CASCADE handles instance_staff + instance_students.
	instancesDeleted, err := s.instanceRepo.DeleteOlderThan(ctx, instanceDateColumn, scheduleModel.Date(cutoff))
	if err != nil {
		return nil, fmt.Errorf("delete activity_instances: %w", err)
	}

	// 4. Delete activity_exceptions.
	exceptionsDeleted, err := s.exceptionRepo.DeleteOlderThan(ctx, exceptionDateColumn, scheduleModel.Date(cutoff))
	if err != nil {
		return nil, fmt.Errorf("delete activity_exceptions: %w", err)
	}

	// 5. Delete audit.deviation_events (#1886). Keyed on occurrence_date —
	// the SAME age predicate the parent instances use — so the protocol for a
	// slot disappears in lockstep with the instances it annotates. No PII
	// beyond IDs, so no data_deletions rows.
	var deviationEventsDeleted int64
	if s.deviationEventRepo != nil {
		deviationEventsDeleted, err = s.deviationEventRepo.DeleteOlderThan(ctx, audit.Date(cutoff))
		if err != nil {
			return nil, fmt.Errorf("delete deviation_events: %w", err)
		}
	}

	result := &TimetableCleanupResult{
		Success:                true,
		InstancesDeleted:       int(instancesDeleted),
		ExceptionsDeleted:      int(exceptionsDeleted),
		DeviationEventsDeleted: int(deviationEventsDeleted),
		StudentsAffected:       len(studentCounts),
		RetentionDays:          retentionDays,
		CutoffDate:             cutoff,
		DurationMS:             time.Since(start).Milliseconds(),
	}

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

// PreviewExpiredTimetableData counts what would be deleted without mutating.
func (s *timetableCleanupService) PreviewExpiredTimetableData(ctx context.Context) (*TimetableCleanupPreview, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("timetable cleanup preview: no tenant in context")
	}

	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := s.cutoffFor(retentionDays)

	instancesToDelete, err := legacyCount(ctx, s.instanceRepo, expiredOptions(instanceDateColumn, cutoff))
	if err != nil {
		return nil, fmt.Errorf("count activity_instances: %w", err)
	}

	exceptionsToDelete, err := legacyCount(ctx, s.exceptionRepo, expiredOptions(exceptionDateColumn, cutoff))
	if err != nil {
		return nil, fmt.Errorf("count activity_exceptions: %w", err)
	}

	studentCounts, _, err := s.collectStudentImpact(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect student impact: %w", err)
	}

	cutoffDate := scheduleModel.Date(cutoff)
	oldestInstance, err := s.instanceRepo.OldestBefore(ctx, instanceDateColumn, &cutoffDate)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_instance: %w", err)
	}
	oldestException, err := s.exceptionRepo.OldestBefore(ctx, exceptionDateColumn, &cutoffDate)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_exception: %w", err)
	}

	return &TimetableCleanupPreview{
		InstancesToDelete:  instancesToDelete,
		ExceptionsToDelete: exceptionsToDelete,
		StudentsAffected:   len(studentCounts),
		RetentionDays:      retentionDays,
		CutoffDate:         cutoff,
		OldestInstance:     cleanupTimezoneDate(oldestInstance),
		OldestException:    cleanupTimezoneDate(oldestException),
	}, nil
}

// GetStats reports total row counts and oldest timestamps regardless of the
// retention window — useful for the CLI `stats` subcommand to gauge overall
// table size.
func (s *timetableCleanupService) GetStats(ctx context.Context) (*TimetableCleanupStats, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("timetable cleanup stats: no tenant in context")
	}

	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := s.cutoffFor(retentionDays)

	totalInstances, err := legacyCount(ctx, s.instanceRepo, nil)
	if err != nil {
		return nil, fmt.Errorf("count activity_instances: %w", err)
	}

	totalExceptions, err := legacyCount(ctx, s.exceptionRepo, nil)
	if err != nil {
		return nil, fmt.Errorf("count activity_exceptions: %w", err)
	}

	oldestInstance, err := s.instanceRepo.OldestBefore(ctx, instanceDateColumn, nil)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_instance: %w", err)
	}
	oldestException, err := s.exceptionRepo.OldestBefore(ctx, exceptionDateColumn, nil)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_exception: %w", err)
	}

	return &TimetableCleanupStats{
		TotalInstances:  totalInstances,
		TotalExceptions: totalExceptions,
		OldestInstance:  cleanupTimezoneDate(oldestInstance),
		OldestException: cleanupTimezoneDate(oldestException),
		RetentionDays:   retentionDays,
		CutoffDate:      cutoff,
	}, nil
}

// --- Internal helpers ---

// resolveRetentionDays picks the tenant's retention days: tenant DB override
// → registry default (both via the settings service). The literal fallback
// is only reached when the settings service is not wired (tests).
//
// Per CLAUDE.md, this service does NOT consult environment variables —
// per-tenant runtime config lives exclusively in the settings system.
func (s *timetableCleanupService) resolveRetentionDays(ctx context.Context) int {
	if s.settings == nil {
		return timetableRetentionDefaultDays
	}
	// Log a warning if the override check fails, but fall through to the
	// registry-default path below so the cleanup still runs on a sane value.
	if _, err := s.settings.HasTenantOverride(ctx, configModel.KeyGDPRTimetableRetentionDays); err != nil {
		s.logger.Warn("settings override check failed, falling back to registry default",
			slog.String("key", configModel.KeyGDPRTimetableRetentionDays),
			slog.String("error", err.Error()),
		)
	}
	// ResolveInt returns the tenant override if set, else the registry default.
	if v, err := s.settings.ResolveInt(ctx, configModel.KeyGDPRTimetableRetentionDays); err == nil && v > 0 {
		return v
	}
	return timetableRetentionDefaultDays
}

// cutoffFor returns the calendar-day cutoff for the retention window.
func (s *timetableCleanupService) cutoffFor(retentionDays int) timezone.Date {
	return s.today().AddDays(-retentionDays)
}

// expiredOptions builds query options selecting rows whose dateColumn is
// strictly before the cutoff.
func expiredOptions(dateColumn string, cutoff timezone.Date) *modelBase.QueryOptions {
	options := modelBase.NewQueryOptions()
	options.Filter = modelBase.NewFilter().LessThan(dateColumn, cutoff)
	return options
}

// studentImpact holds the per-student count and a bounded sample of instance
// IDs for the audit metadata.
type perStudentCounts map[int64]int
type perStudentSamples map[int64][]int64

func cleanupTimezoneDate(date *scheduleModel.Date) *timezone.Date {
	if date == nil {
		return nil
	}
	converted := timezone.Date(*date)
	return &converted
}

// collectStudentImpact returns per-student counts of instance_students rows
// that will be cleaned up (by way of CASCADE from activity_instances), plus
// a bounded sample of instance IDs per student for forensic lookup.
func (s *timetableCleanupService) collectStudentImpact(
	ctx context.Context,
	cutoff timezone.Date,
) (perStudentCounts, perStudentSamples, error) {
	refs, err := s.instanceStudentRepo.ListStudentInstanceRefsBefore(ctx, scheduleModel.Date(cutoff))
	if err != nil {
		return nil, nil, err
	}

	counts := make(perStudentCounts)
	samples := make(perStudentSamples)
	for _, ref := range refs {
		counts[ref.StudentID]++
		if len(samples[ref.StudentID]) < auditInstanceIDSampleCap {
			samples[ref.StudentID] = append(samples[ref.StudentID], ref.InstanceID)
		}
	}
	return counts, samples, nil
}

// writeStudentAuditRows inserts one audit.data_deletions row per affected
// student. Called BEFORE the deletes so any failure rolls back everything.
func (s *timetableCleanupService) writeStudentAuditRows(
	ctx context.Context,
	tenantID int64,
	retentionDays int,
	cutoff timezone.Date,
	counts perStudentCounts,
	samples perStudentSamples,
) error {
	if s.auditRepo == nil {
		return fmt.Errorf("audit repo not configured")
	}
	cutoffStr := cutoff.String()
	for studentID, n := range counts {
		deletion := audit.NewDataDeletion(
			studentID,
			audit.DeletionTypeTimetableRetention,
			n,
			"system",
		)
		deletion.SetTenantID(tenantID)
		deletion.DeletionReason = "automated timetable retention cleanup"
		deletion.SetMetadata("retention_days", retentionDays)
		deletion.SetMetadata("cutoff_date", cutoffStr)
		if sample := samples[studentID]; len(sample) > 0 {
			deletion.SetMetadata("instance_ids_sample", sample)
		}
		if err := s.auditRepo.Create(ctx, deletion); err != nil {
			return fmt.Errorf("audit row for student %d: %w", studentID, err)
		}
	}
	return nil
}
