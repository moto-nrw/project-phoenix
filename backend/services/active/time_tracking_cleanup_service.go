// Package active contains the time-tracking cleanup service (Tranche 0b).
//
// GDPR retention cleanup for the time-tracking tables. Deletes rows older
// than the tenant-configured retention window. Built to satisfy:
//   - §16 Abs. 2 ArbZG (2-year minimum for Arbeitszeit-Nachweise)
//   - DSGVO Art. 5 lit. e (Speicherbegrenzung, no longer than needed)
//   - §41 EStG / §147 AO (when payroll-relevant, admins raise retention)
//
// Design notes:
//
//   - Two DELETE statements per tenant run:
//     1. active.work_sessions WHERE date < cutoff
//     CASCADE removes active.work_session_breaks and
//     audit.work_session_edits via existing FK constraints.
//     2. active.staff_absences WHERE date_end < cutoff
//     No children, independent table.
//
//   - The caller establishes tenant context (WithTenantTx). RLS is the
//     primary tenant boundary; the repositories add tenant_id predicates
//     as defense in depth.
//
//   - Per-staff audit rows in audit.data_deletions, one row per affected
//     staff member (not per deleted row). Uses the staff_id subject
//     column added in migration 1.15.58. Audit rows are written BEFORE
//     the deletes inside the caller's transaction. If either the audit
//     write or the delete fails, everything rolls back.
//
//   - Retention resolution: tenant DB override, registry default,
//     last-resort constant. The constant is only reached in tests where
//     the settings service isn't wired.
package active

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// workSessionDateColumn / staffAbsenceDateColumn are the retention-age
// columns the cleanup predicates run on.
const (
	workSessionDateColumn  = "date"
	staffAbsenceDateColumn = "date_end"
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

// TimeTrackingCleanupPreview is what PreviewExpiredTimeTrackingData returns,
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
	workSessionRepo  activeModel.WorkSessionRepository
	staffAbsenceRepo activeModel.StaffAbsenceRepository
	auditRepo        audit.DataDeletionRepository
	settings         config.SettingsService
	logger           *slog.Logger
}

// NewTimeTrackingCleanupService constructs the cleanup service. auditRepo
// receives a per-staff data_deletions row per cleanup run; settings may be
// nil in tests, in which case retention resolves to the last-resort default.
func NewTimeTrackingCleanupService(
	workSessionRepo activeModel.WorkSessionRepository,
	staffAbsenceRepo activeModel.StaffAbsenceRepository,
	auditRepo audit.DataDeletionRepository,
	settings config.SettingsService,
	logger *slog.Logger,
) TimeTrackingCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &timeTrackingCleanupService{
		workSessionRepo:  workSessionRepo,
		staffAbsenceRepo: staffAbsenceRepo,
		auditRepo:        auditRepo,
		settings:         settings,
		logger:           logger,
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

	// 1. Collect per-staff impact for the audit log. We need this before
	//    the deletes so the audit rows can be written first.
	staffCounts, sampleIDs, err := s.collectStaffImpact(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect staff impact: %w", err)
	}

	// 2. Write one audit row per affected staff BEFORE the deletes.
	if err := s.writeStaffAuditRows(ctx, tenantID, retentionDays, cutoff, staffCounts, sampleIDs); err != nil {
		return nil, fmt.Errorf("write audit rows: %w", err)
	}

	// 3. Delete work_sessions (CASCADE removes breaks + audit edits).
	sessionsDeleted, err := s.workSessionRepo.DeleteOlderThan(ctx, workSessionDateColumn, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete work_sessions: %w", err)
	}

	// 4. Delete staff_absences (independent table).
	absencesDeleted, err := s.staffAbsenceRepo.DeleteOlderThan(ctx, staffAbsenceDateColumn, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete staff_absences: %w", err)
	}

	result := &TimeTrackingCleanupResult{
		Success:         true,
		SessionsDeleted: int(sessionsDeleted),
		AbsencesDeleted: int(absencesDeleted),
		StaffAffected:   len(staffCounts),
		RetentionDays:   retentionDays,
		CutoffDate:      cutoff,
		DurationMS:      time.Since(start).Milliseconds(),
	}

	s.logger.Info("time-tracking cleanup completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int("sessions_deleted", result.SessionsDeleted),
		slog.Int("absences_deleted", result.AbsencesDeleted),
		slog.Int("staff_affected", result.StaffAffected),
		slog.Int("retention_days", retentionDays),
		slog.String("cutoff", cutoff.Format("2006-01-02")),
		slog.Int64("duration_ms", result.DurationMS),
	)

	return result, nil
}

// PreviewExpiredTimeTrackingData runs the same queries CleanupExpired would,
// but only counts. Nothing is deleted. Used by the CLI --dry-run flag.
func (s *timeTrackingCleanupService) PreviewExpiredTimeTrackingData(ctx context.Context) (*TimeTrackingCleanupPreview, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("time-tracking cleanup preview: no tenant in context")
	}

	retentionDays := s.resolveRetentionDays(ctx)
	cutoff := cutoffForDays(retentionDays)

	sessionsToDelete, err := s.workSessionRepo.CountWithOptions(ctx, olderThanOptions(workSessionDateColumn, cutoff))
	if err != nil {
		return nil, fmt.Errorf("count work_sessions: %w", err)
	}
	absencesToDelete, err := s.staffAbsenceRepo.CountWithOptions(ctx, olderThanOptions(staffAbsenceDateColumn, cutoff))
	if err != nil {
		return nil, fmt.Errorf("count staff_absences: %w", err)
	}
	// Preview shares the impact-collection helper used by the real run so
	// "what would happen" can never drift from "what actually happens".
	staffCounts, _, err := s.collectStaffImpact(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("collect staff impact: %w", err)
	}
	staffAffected := len(staffCounts)

	oldestSession, err := s.workSessionRepo.OldestBefore(ctx, workSessionDateColumn, &cutoff)
	if err != nil {
		return nil, fmt.Errorf("oldest work_session: %w", err)
	}
	oldestAbsence, err := s.staffAbsenceRepo.OldestBefore(ctx, staffAbsenceDateColumn, &cutoff)
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

	totalSessions, err := s.workSessionRepo.CountWithOptions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("count sessions: %w", err)
	}
	totalAbsences, err := s.staffAbsenceRepo.CountWithOptions(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("count absences: %w", err)
	}

	oldestSession, err := s.workSessionRepo.OldestBefore(ctx, workSessionDateColumn, nil)
	if err != nil {
		return nil, fmt.Errorf("oldest session: %w", err)
	}
	oldestAbsence, err := s.staffAbsenceRepo.OldestBefore(ctx, staffAbsenceDateColumn, nil)
	if err != nil {
		return nil, fmt.Errorf("oldest absence: %w", err)
	}

	return &TimeTrackingCleanupStats{
		TotalSessions: totalSessions,
		TotalAbsences: totalAbsences,
		OldestSession: oldestSession,
		OldestAbsence: oldestAbsence,
		RetentionDays: retentionDays,
		CutoffDate:    cutoff,
	}, nil
}

// --- Internal helpers ---

// resolveRetentionDays picks the tenant's retention days: tenant DB override
// or registry default. The literal fallback is only reached when the
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
// today minus retentionDays will be deleted.
func cutoffForDays(retentionDays int) time.Time {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return today.AddDate(0, 0, -retentionDays)
}

// olderThanOptions builds query options selecting rows whose dateColumn is
// strictly before the cutoff.
func olderThanOptions(dateColumn string, cutoff time.Time) *modelBase.QueryOptions {
	options := modelBase.NewQueryOptions()
	options.Filter = modelBase.NewFilter().LessThan(dateColumn, cutoff)
	return options
}

// staffImpactOptions extends olderThanOptions with the (staff_id, id)
// ordering the audit sample bookkeeping relies on.
func staffImpactOptions(dateColumn string, cutoff time.Time) *modelBase.QueryOptions {
	options := olderThanOptions(dateColumn, cutoff)
	sorting := &modelBase.Sorting{}
	sorting.AddField("staff_id", modelBase.SortAsc).AddField("id", modelBase.SortAsc)
	options.Sorting = sorting
	return options
}

// perStaffCounts maps staff_id to the number of rows older than cutoff (sessions
// + absences combined). Used for one audit row per affected staff member.
type perStaffCounts map[int64]int

// perStaffSamples maps staff_id to a bounded sample of session/absence IDs for
// audit metadata. Lets compliance back-trace specific rows after the fact
// without bloating the JSONB column.
type perStaffSamples map[int64]perStaffSampleIDs

type perStaffSampleIDs struct {
	SessionIDs []int64
	AbsenceIDs []int64
}

// collectStaffImpact returns per-staff counts of sessions and absences that
// will be deleted, plus a bounded sample of their IDs. Two repository reads
// (one per table); tenant scoping comes from the repositories on top of the
// caller's RLS transaction.
func (s *timeTrackingCleanupService) collectStaffImpact(
	ctx context.Context,
	cutoff time.Time,
) (perStaffCounts, perStaffSamples, error) {
	counts := make(perStaffCounts)
	samples := make(perStaffSamples)

	sessions, err := s.workSessionRepo.List(ctx, staffImpactOptions(workSessionDateColumn, cutoff))
	if err != nil {
		return nil, nil, fmt.Errorf("scan work_sessions: %w", err)
	}
	for _, session := range sessions {
		counts[session.StaffID]++
		entry := samples[session.StaffID]
		if len(entry.SessionIDs) < auditStaffIDSampleCap {
			entry.SessionIDs = append(entry.SessionIDs, session.ID)
			samples[session.StaffID] = entry
		}
	}

	absences, err := s.staffAbsenceRepo.List(ctx, staffImpactOptions(staffAbsenceDateColumn, cutoff))
	if err != nil {
		return nil, nil, fmt.Errorf("scan staff_absences: %w", err)
	}
	for _, absence := range absences {
		counts[absence.StaffID]++
		entry := samples[absence.StaffID]
		if len(entry.AbsenceIDs) < auditStaffIDSampleCap {
			entry.AbsenceIDs = append(entry.AbsenceIDs, absence.ID)
			samples[absence.StaffID] = entry
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
