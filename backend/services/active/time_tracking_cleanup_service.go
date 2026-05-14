// Package active — time-tracking cleanup service (Tranche 0b).
//
// GDPR retention cleanup for the time-tracking tables. Deletes rows older
// than the tenant-configured retention window. Built to satisfy:
//   - §16 Abs. 2 ArbZG (2-year minimum for Arbeitszeit-Nachweise)
//   - DSGVO Art. 5 lit. e (Speicherbegrenzung — no longer than needed)
//   - §41 EStG / §147 AO (when payroll-relevant — admins raise retention)
//
// Design notes:
//
//   - Two DELETE statements per tenant run:
//     1. active.work_sessions WHERE created_at < cutoff
//     → CASCADE removes active.work_session_breaks and
//     audit.work_session_edits via existing FK constraints.
//     2. active.staff_absences WHERE created_at < cutoff
//     → no children, independent table.
//
//   - The caller establishes tenant context (WithTenantTx). RLS is the
//     primary tenant boundary; explicit tenant_id = ? predicates are
//     defense-in-depth.
//
//   - Per-staff audit rows in audit.data_deletions, one row per affected
//     staff member (not per deleted row). Uses the staff_id subject
//     column added in migration 1.15.58. Audit rows are written BEFORE
//     the deletes inside the caller's transaction — if either the audit
//     write or the delete fails, everything rolls back.
//
//   - Retention resolution: tenant DB override → registry default →
//     last-resort constant. The constant is only reached in tests where
//     the settings service isn't wired.
package active

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

// timeTrackingRetentionDefaultDays is a last-resort fallback. In production
// this is unreachable because services/config/defaults/gdpr.go registers
// 730 at init time. Kept so the service is callable in tests without
// settings wiring.
const timeTrackingRetentionDefaultDays = 730

// auditStaffIDSampleCap bounds the number of session/absence IDs sampled
// into each per-staff audit row's metadata. Keeps the metadata JSONB
// compact while still giving compliance a handful of identifiers for
// forensic lookup.
const auditStaffIDSampleCap = 10

// TimeTrackingCleanupResult summarises one cleanup run for a single tenant.
type TimeTrackingCleanupResult struct {
	Success         bool
	SessionsDeleted int
	AbsencesDeleted int
	StaffAffected   int
	RetentionDays   int
	CutoffDate      time.Time
	DurationMS      int64
}

// TimeTrackingCleanupPreview is what PreviewExpiredTimeTrackingData returns —
// the same numbers as a Result, but nothing was actually deleted.
type TimeTrackingCleanupPreview struct {
	SessionsToDelete int
	AbsencesToDelete int
	StaffAffected    int
	RetentionDays    int
	CutoffDate       time.Time
	OldestSession    *time.Time
	OldestAbsence    *time.Time
}

// TimeTrackingCleanupStats returns the current state of the time-tracking
// tables for a tenant. Used by the CLI `stats` subcommand.
type TimeTrackingCleanupStats struct {
	TotalSessions int
	TotalAbsences int
	OldestSession *time.Time
	OldestAbsence *time.Time
	RetentionDays int
	CutoffDate    time.Time
}

// TimeTrackingCleanupService drives GDPR retention cleanup for the
// time-tracking tables. All methods assume tenant context has been
// established by the caller (WithTenantTx).
type TimeTrackingCleanupService interface {
	CleanupExpiredTimeTrackingData(ctx context.Context) (*TimeTrackingCleanupResult, error)
	PreviewExpiredTimeTrackingData(ctx context.Context) (*TimeTrackingCleanupPreview, error)
	GetStats(ctx context.Context) (*TimeTrackingCleanupStats, error)
}

type timeTrackingCleanupService struct {
	db        *bun.DB
	auditRepo audit.DataDeletionRepository
	settings  config.SettingsService
	logger    *slog.Logger
}

// NewTimeTrackingCleanupService constructs the cleanup service. auditRepo
// receives a per-staff data_deletions row per cleanup run; settings may be
// nil in tests, in which case retention resolves to the last-resort default.
func NewTimeTrackingCleanupService(
	db *bun.DB,
	auditRepo audit.DataDeletionRepository,
	settings config.SettingsService,
	logger *slog.Logger,
) TimeTrackingCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &timeTrackingCleanupService{
		db:        db,
		auditRepo: auditRepo,
		settings:  settings,
		logger:    logger,
	}
}

// CleanupExpiredTimeTrackingData deletes work_sessions and staff_absences
// older than the resolved retention window. Returns the per-table counts
// and the cutoff that was applied. Writes one audit.data_deletions row per
// affected staff member BEFORE the deletes, so any failure rolls back
// everything atomically.
func (s *timeTrackingCleanupService) CleanupExpiredTimeTrackingData(ctx context.Context) (*TimeTrackingCleanupResult, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("time-tracking cleanup: no tenant in context")
	}

	start := time.Now()
	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := cutoffForDays(retentionDays)
	db := repoBase.GetDB(ctx, s.db)

	// 1. Collect per-staff impact for the audit log. We need this before
	//    the deletes so the audit rows can be written first.
	staffCounts, sampleIDs, err := s.collectStaffImpact(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect staff impact: %w", err)
	}

	// 2. Write one audit row per affected staff BEFORE the deletes.
	if err := s.writeStaffAuditRows(ctx, tenantID, retentionDays, cutoff, staffCounts, sampleIDs); err != nil {
		return nil, fmt.Errorf("write audit rows: %w", err)
	}

	// 3. Delete work_sessions (CASCADE removes breaks + audit edits).
	sessionsDeleted, err := deleteOldWorkSessions(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete work_sessions: %w", err)
	}

	// 4. Delete staff_absences (independent table).
	absencesDeleted, err := deleteOldStaffAbsences(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete staff_absences: %w", err)
	}

	result := &TimeTrackingCleanupResult{
		Success:         true,
		SessionsDeleted: sessionsDeleted,
		AbsencesDeleted: absencesDeleted,
		StaffAffected:   len(staffCounts),
		RetentionDays:   retentionDays,
		CutoffDate:      cutoff,
		DurationMS:      time.Since(start).Milliseconds(),
	}

	s.logger.Info("time-tracking cleanup completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int("sessions_deleted", sessionsDeleted),
		slog.Int("absences_deleted", absencesDeleted),
		slog.Int("staff_affected", result.StaffAffected),
		slog.Int("retention_days", retentionDays),
		slog.String("cutoff", cutoff.Format("2006-01-02")),
		slog.Int64("duration_ms", result.DurationMS),
	)

	return result, nil
}

// PreviewExpiredTimeTrackingData runs the same queries CleanupExpired would,
// but only counts — nothing is deleted. Used by the CLI --dry-run flag.
func (s *timeTrackingCleanupService) PreviewExpiredTimeTrackingData(ctx context.Context) (*TimeTrackingCleanupPreview, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("time-tracking cleanup preview: no tenant in context")
	}

	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := cutoffForDays(retentionDays)
	db := repoBase.GetDB(ctx, s.db)

	sessionsToDelete, err := countOldWorkSessions(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("count work_sessions: %w", err)
	}
	absencesToDelete, err := countOldStaffAbsences(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("count staff_absences: %w", err)
	}
	// Preview shares the impact-collection helper used by the real run so
	// "what would happen" can never drift from "what actually happens".
	staffCounts, _, err := s.collectStaffImpact(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect staff impact: %w", err)
	}
	staffAffected := len(staffCounts)

	oldestSession, err := oldestWorkSession(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("oldest work_session: %w", err)
	}
	oldestAbsence, err := oldestStaffAbsence(ctx, db, tenantID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("oldest staff_absence: %w", err)
	}

	return &TimeTrackingCleanupPreview{
		SessionsToDelete: sessionsToDelete,
		AbsencesToDelete: absencesToDelete,
		StaffAffected:    staffAffected,
		RetentionDays:    retentionDays,
		CutoffDate:       cutoff,
		OldestSession:    oldestSession,
		OldestAbsence:    oldestAbsence,
	}, nil
}

// GetStats returns the unfiltered current size of the time-tracking tables
// for the tenant. CLI uses it to make "you have N old rows" reports.
func (s *timeTrackingCleanupService) GetStats(ctx context.Context) (*TimeTrackingCleanupStats, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("time-tracking cleanup stats: no tenant in context")
	}

	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := cutoffForDays(retentionDays)
	db := repoBase.GetDB(ctx, s.db)

	type row struct {
		Cnt int `bun:"cnt"`
	}
	var sessionRow row
	if err := db.NewSelect().
		TableExpr("active.work_sessions").
		ColumnExpr("COUNT(*) AS cnt").
		Where("tenant_id = ?", tenantID).
		Scan(ctx, &sessionRow); err != nil {
		return nil, fmt.Errorf("count sessions: %w", err)
	}
	var absenceRow row
	if err := db.NewSelect().
		TableExpr("active.staff_absences").
		ColumnExpr("COUNT(*) AS cnt").
		Where("tenant_id = ?", tenantID).
		Scan(ctx, &absenceRow); err != nil {
		return nil, fmt.Errorf("count absences: %w", err)
	}

	oldestSession, err := oldestWorkSession(ctx, db, tenantID, time.Now().AddDate(100, 0, 0))
	if err != nil {
		return nil, fmt.Errorf("oldest session: %w", err)
	}
	oldestAbsence, err := oldestStaffAbsence(ctx, db, tenantID, time.Now().AddDate(100, 0, 0))
	if err != nil {
		return nil, fmt.Errorf("oldest absence: %w", err)
	}

	return &TimeTrackingCleanupStats{
		TotalSessions: sessionRow.Cnt,
		TotalAbsences: absenceRow.Cnt,
		OldestSession: oldestSession,
		OldestAbsence: oldestAbsence,
		RetentionDays: retentionDays,
		CutoffDate:    cutoff,
	}, nil
}

// --- Internal helpers ---

// resolveRetentionDays picks the tenant's retention days: tenant DB override
// → registry default. The literal fallback is only reached when the
// settings service is not wired (tests).
func (s *timeTrackingCleanupService) resolveRetentionDays(ctx context.Context) int {
	if s.settings == nil {
		return timeTrackingRetentionDefaultDays
	}
	if _, err := s.settings.HasTenantOverride(ctx, configModel.KeyGDPRTimeTrackingRetentionDays); err != nil {
		s.logger.Warn("settings override check failed, falling back to registry default",
			slog.String("key", configModel.KeyGDPRTimeTrackingRetentionDays),
			slog.String("error", err.Error()),
		)
	}
	if v, err := s.settings.ResolveInt(ctx, configModel.KeyGDPRTimeTrackingRetentionDays); err == nil && v > 0 {
		return v
	}
	return timeTrackingRetentionDefaultDays
}

// cutoffForDays returns the UTC start-of-day cutoff: anything older than
// today − retentionDays will be deleted.
func cutoffForDays(retentionDays int) time.Time {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -retentionDays)
}

// perStaffCounts maps staff_id → number of rows older than cutoff (sessions
// + absences combined). Used for one audit row per affected staff member.
type perStaffCounts map[int64]int

// perStaffSamples maps staff_id → bounded sample of session/absence IDs for
// audit metadata. Lets compliance back-trace specific rows after the fact
// without bloating the JSONB column.
type perStaffSamples map[int64]perStaffSampleIDs

type perStaffSampleIDs struct {
	SessionIDs []int64
	AbsenceIDs []int64
}

// collectStaffImpact returns per-staff counts of sessions and absences that
// will be deleted, plus a bounded sample of their IDs. Two SELECTs (one per
// table) keep the SQL readable; both are tenant-scoped explicitly even
// though RLS would also enforce it.
func (s *timeTrackingCleanupService) collectStaffImpact(
	ctx context.Context,
	db bun.IDB,
	tenantID int64,
	cutoff time.Time,
) (perStaffCounts, perStaffSamples, error) {
	counts := make(perStaffCounts)
	samples := make(perStaffSamples)

	type sessionRow struct {
		StaffID   int64 `bun:"staff_id"`
		SessionID int64 `bun:"id"`
	}
	var sessionRows []sessionRow
	if err := db.NewSelect().
		TableExpr("active.work_sessions AS w").
		ColumnExpr("w.staff_id AS staff_id").
		ColumnExpr("w.id AS id").
		Where("w.tenant_id = ?", tenantID).
		Where("w.created_at < ?", cutoff).
		Order("w.staff_id", "w.id").
		Scan(ctx, &sessionRows); err != nil {
		return nil, nil, fmt.Errorf("scan work_sessions: %w", err)
	}
	for _, r := range sessionRows {
		counts[r.StaffID]++
		entry := samples[r.StaffID]
		if len(entry.SessionIDs) < auditStaffIDSampleCap {
			entry.SessionIDs = append(entry.SessionIDs, r.SessionID)
			samples[r.StaffID] = entry
		}
	}

	type absenceRow struct {
		StaffID   int64 `bun:"staff_id"`
		AbsenceID int64 `bun:"id"`
	}
	var absenceRows []absenceRow
	if err := db.NewSelect().
		TableExpr("active.staff_absences AS a").
		ColumnExpr("a.staff_id AS staff_id").
		ColumnExpr("a.id AS id").
		Where("a.tenant_id = ?", tenantID).
		Where("a.created_at < ?", cutoff).
		Order("a.staff_id", "a.id").
		Scan(ctx, &absenceRows); err != nil {
		return nil, nil, fmt.Errorf("scan staff_absences: %w", err)
	}
	for _, r := range absenceRows {
		counts[r.StaffID]++
		entry := samples[r.StaffID]
		if len(entry.AbsenceIDs) < auditStaffIDSampleCap {
			entry.AbsenceIDs = append(entry.AbsenceIDs, r.AbsenceID)
			samples[r.StaffID] = entry
		}
	}

	return counts, samples, nil
}

// writeStaffAuditRows inserts one audit.data_deletions row per affected
// staff member. Called BEFORE the deletes so any failure rolls back
// everything atomically (caller-supplied WithTenantTx).
func (s *timeTrackingCleanupService) writeStaffAuditRows(
	ctx context.Context,
	tenantID int64,
	retentionDays int,
	cutoff time.Time,
	counts perStaffCounts,
	samples perStaffSamples,
) error {
	if s.auditRepo == nil {
		return fmt.Errorf("audit repo not configured")
	}
	cutoffStr := cutoff.Format("2006-01-02")
	for staffID, n := range counts {
		deletion := audit.NewStaffDataDeletion(
			staffID,
			audit.DeletionTypeTimeTrackingRetention,
			n,
			"system",
		)
		deletion.SetTenantID(tenantID)
		deletion.DeletionReason = "automated time-tracking retention cleanup"
		deletion.SetMetadata("retention_days", retentionDays)
		deletion.SetMetadata("cutoff_date", cutoffStr)
		if sample := samples[staffID]; len(sample.SessionIDs) > 0 {
			deletion.SetMetadata("session_ids_sample", sample.SessionIDs)
		}
		if sample := samples[staffID]; len(sample.AbsenceIDs) > 0 {
			deletion.SetMetadata("absence_ids_sample", sample.AbsenceIDs)
		}
		if err := s.auditRepo.Create(ctx, deletion); err != nil {
			return fmt.Errorf("audit row for staff %d: %w", staffID, err)
		}
	}
	return nil
}

func deleteOldWorkSessions(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (int, error) {
	res, err := db.NewDelete().
		Table("active.work_sessions").
		Where("tenant_id = ?", tenantID).
		Where("created_at < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func deleteOldStaffAbsences(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (int, error) {
	res, err := db.NewDelete().
		Table("active.staff_absences").
		Where("tenant_id = ?", tenantID).
		Where("created_at < ?", cutoff).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func countOldWorkSessions(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (int, error) {
	type row struct {
		Cnt int `bun:"cnt"`
	}
	var r row
	err := db.NewSelect().
		TableExpr("active.work_sessions").
		ColumnExpr("COUNT(*) AS cnt").
		Where("tenant_id = ?", tenantID).
		Where("created_at < ?", cutoff).
		Scan(ctx, &r)
	return r.Cnt, err
}

func countOldStaffAbsences(ctx context.Context, db bun.IDB, tenantID int64, cutoff time.Time) (int, error) {
	type row struct {
		Cnt int `bun:"cnt"`
	}
	var r row
	err := db.NewSelect().
		TableExpr("active.staff_absences").
		ColumnExpr("COUNT(*) AS cnt").
		Where("tenant_id = ?", tenantID).
		Where("created_at < ?", cutoff).
		Scan(ctx, &r)
	return r.Cnt, err
}

func oldestWorkSession(ctx context.Context, db bun.IDB, tenantID int64, _ time.Time) (*time.Time, error) {
	type row struct {
		CreatedAt *time.Time `bun:"created_at"`
	}
	var r row
	err := db.NewSelect().
		TableExpr("active.work_sessions").
		ColumnExpr("MIN(created_at) AS created_at").
		Where("tenant_id = ?", tenantID).
		Scan(ctx, &r)
	if err != nil {
		return nil, err
	}
	return r.CreatedAt, nil
}

func oldestStaffAbsence(ctx context.Context, db bun.IDB, tenantID int64, _ time.Time) (*time.Time, error) {
	type row struct {
		CreatedAt *time.Time `bun:"created_at"`
	}
	var r row
	err := db.NewSelect().
		TableExpr("active.staff_absences").
		ColumnExpr("MIN(created_at) AS created_at").
		Where("tenant_id = ?", tenantID).
		Scan(ctx, &r)
	if err != nil {
		return nil, err
	}
	return r.CreatedAt, nil
}
