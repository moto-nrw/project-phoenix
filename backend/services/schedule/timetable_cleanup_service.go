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
//     RLS enforces the tenant boundary. Explicit `tenant_id = ?` predicates
//     are defense-in-depth.
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

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
	Success           bool
	InstancesDeleted  int
	ExceptionsDeleted int
	StudentsAffected  int
	RetentionDays     int
	CutoffDate        time.Time
	DurationMS        int64
}

// TimetableCleanupPreview is returned by PreviewExpiredTimetableData —
// identical shape to Result except nothing was deleted.
type TimetableCleanupPreview struct {
	InstancesToDelete  int
	ExceptionsToDelete int
	StudentsAffected   int
	RetentionDays      int
	CutoffDate         time.Time
	OldestInstance     *time.Time
	OldestException    *time.Time
}

// TimetableCleanupStats returns row counts and oldest timestamps across the
// tenant's timetable data for the CLI `stats` subcommand.
type TimetableCleanupStats struct {
	TotalInstances  int
	TotalExceptions int
	OldestInstance  *time.Time
	OldestException *time.Time
	RetentionDays   int
	CutoffDate      time.Time
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
	db        *bun.DB
	auditRepo audit.DataDeletionRepository
	settings  config.SettingsService
	logger    *slog.Logger
}

// NewTimetableCleanupService constructs the cleanup service. settings may be
// nil in tests; when nil, retention resolves to the last-resort default 365.
func NewTimetableCleanupService(
	db *bun.DB,
	auditRepo audit.DataDeletionRepository,
	settings config.SettingsService,
	logger *slog.Logger,
) TimetableCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &timetableCleanupService{
		db:        db,
		auditRepo: auditRepo,
		settings:  settings,
		logger:    logger,
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
	cutoff := cutoffFor(retentionDays)

	db := repoBase.GetDB(ctx, s.db)

	// 1. Collect per-student impact for the audit log.
	studentCounts, sampleIDs, err := s.collectStudentImpact(ctx, db, tenantID, cutoff)
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
	instancesDeleted, err := deleteOldInstances(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete activity_instances: %w", err)
	}

	// 4. Delete activity_exceptions.
	exceptionsDeleted, err := deleteOldExceptions(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete activity_exceptions: %w", err)
	}

	result := &TimetableCleanupResult{
		Success:           true,
		InstancesDeleted:  instancesDeleted,
		ExceptionsDeleted: exceptionsDeleted,
		StudentsAffected:  len(studentCounts),
		RetentionDays:     retentionDays,
		CutoffDate:        cutoff,
		DurationMS:        time.Since(start).Milliseconds(),
	}

	s.logger.Info("timetable cleanup completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int("instances_deleted", instancesDeleted),
		slog.Int("exceptions_deleted", exceptionsDeleted),
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
	cutoff := cutoffFor(retentionDays)
	db := repoBase.GetDB(ctx, s.db)

	instancesToDelete, err := db.NewSelect().
		Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).
		Where("date < ?", cutoff).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count activity_instances: %w", err)
	}

	exceptionsToDelete, err := db.NewSelect().
		Table("schedule.activity_exceptions").
		Where("tenant_id = ?", tenantID).
		Where("exception_date < ?", cutoff).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count activity_exceptions: %w", err)
	}

	studentCounts, _, err := s.collectStudentImpact(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect student impact: %w", err)
	}

	oldestInstance, err := scanOldestTimestamp(ctx, db,
		`SELECT MIN(date) AS t FROM schedule.activity_instances WHERE tenant_id = ? AND date < ?`,
		tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_instance: %w", err)
	}
	oldestException, err := scanOldestTimestamp(ctx, db,
		`SELECT MIN(exception_date) AS t FROM schedule.activity_exceptions WHERE tenant_id = ? AND exception_date < ?`,
		tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_exception: %w", err)
	}

	return &TimetableCleanupPreview{
		InstancesToDelete:  instancesToDelete,
		ExceptionsToDelete: exceptionsToDelete,
		StudentsAffected:   len(studentCounts),
		RetentionDays:      retentionDays,
		CutoffDate:         cutoff,
		OldestInstance:     oldestInstance,
		OldestException:    oldestException,
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
	cutoff := cutoffFor(retentionDays)
	db := repoBase.GetDB(ctx, s.db)

	totalInstances, err := db.NewSelect().
		Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count activity_instances: %w", err)
	}

	totalExceptions, err := db.NewSelect().
		Table("schedule.activity_exceptions").
		Where("tenant_id = ?", tenantID).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("count activity_exceptions: %w", err)
	}

	oldestInstance, err := scanOldestTimestamp(ctx, db,
		`SELECT MIN(date) AS t FROM schedule.activity_instances WHERE tenant_id = ?`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_instance: %w", err)
	}
	oldestException, err := scanOldestTimestamp(ctx, db,
		`SELECT MIN(exception_date) AS t FROM schedule.activity_exceptions WHERE tenant_id = ?`,
		tenantID)
	if err != nil {
		return nil, fmt.Errorf("oldest activity_exception: %w", err)
	}

	return &TimetableCleanupStats{
		TotalInstances:  totalInstances,
		TotalExceptions: totalExceptions,
		OldestInstance:  oldestInstance,
		OldestException: oldestException,
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

// cutoffFor returns the UTC start-of-day cutoff for the retention window.
func cutoffFor(retentionDays int) time.Time {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -retentionDays)
}

// studentImpact holds the per-student count and a bounded sample of instance
// IDs for the audit metadata.
type perStudentCounts map[int64]int
type perStudentSamples map[int64][]int64

// collectStudentImpact returns per-student counts of instance_students rows
// that will be cleaned up (by way of CASCADE from activity_instances), plus
// a bounded sample of instance IDs per student for forensic lookup.
func (s *timetableCleanupService) collectStudentImpact(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	cutoff time.Time,
) (perStudentCounts, perStudentSamples, error) {
	type row struct {
		StudentID  int64 `bun:"student_id"`
		InstanceID int64 `bun:"instance_id"`
	}
	var rows []row
	err := db.NewSelect().
		TableExpr("schedule.instance_students AS i_s").
		ColumnExpr("i_s.student_id AS student_id").
		ColumnExpr("i_s.instance_id AS instance_id").
		Join(`JOIN schedule.activity_instances AS i ON i.id = i_s.instance_id`).
		Where("i.tenant_id = ?", tenantID).
		Where("i.date < ?", cutoff).
		Order("i_s.student_id", "i_s.instance_id").
		Scan(ctx, &rows)
	if err != nil {
		return nil, nil, err
	}

	counts := make(perStudentCounts)
	samples := make(perStudentSamples)
	for _, r := range rows {
		counts[r.StudentID]++
		if len(samples[r.StudentID]) < auditInstanceIDSampleCap {
			samples[r.StudentID] = append(samples[r.StudentID], r.InstanceID)
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
	cutoff time.Time,
	counts perStudentCounts,
	samples perStudentSamples,
) error {
	if s.auditRepo == nil {
		return fmt.Errorf("audit repo not configured")
	}
	cutoffStr := cutoff.Format("2006-01-02")
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

// deleteOldInstances issues the DELETE and returns the affected count.
func deleteOldInstances(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (int, error) {
	res, err := db.NewDelete().
		Table("schedule.activity_instances").
		Where("tenant_id = ?", tenantID).
		Where("date < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

// deleteOldExceptions mirrors deleteOldInstances for exceptions.
func deleteOldExceptions(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (int, error) {
	res, err := db.NewDelete().
		Table("schedule.activity_exceptions").
		Where("tenant_id = ?", tenantID).
		Where("exception_date < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

// scanOldestTimestamp runs a MIN() query and returns nil when no rows match.
// The caller must include `AS t` on the MIN() column so bun can map it.
func scanOldestTimestamp(ctx context.Context, db bun.IDB, query string, args ...any) (*time.Time, error) {
	var nt struct {
		T *time.Time `bun:"t"`
	}
	err := db.NewRaw(query, args...).Scan(ctx, &nt)
	if err != nil {
		return nil, err
	}
	return nt.T, nil
}
