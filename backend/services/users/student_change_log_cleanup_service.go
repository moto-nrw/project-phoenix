package users

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// studentChangeLogRetentionDefaultDays keeps the service callable in tests
// without settings wiring. Production always injects the settings service.
const studentChangeLogRetentionDefaultDays = 90

// StudentChangeLogCleanupResult summarises one cleanup run for a single tenant.
type StudentChangeLogCleanupResult struct {
	Success          bool
	EditsDeleted     int
	StudentsAffected int
	RetentionDays    int
	Cutoff           time.Time
	DurationMS       int64
}

// StudentChangeLogCleanupService drives GDPR retention cleanup for the
// per-child change history (audit.student_field_edits, issue #1455). All
// methods assume tenant context has been established by the caller
// (WithTenantTx) — the scheduler does this per tenant.
type StudentChangeLogCleanupService interface {
	CleanupExpiredChangeLog(ctx context.Context) (*StudentChangeLogCleanupResult, error)
}

type studentChangeLogCleanupService struct {
	repo      auditModels.StudentFieldEditRepository
	auditRepo auditModels.DataDeletionRepository
	settings  config.SettingsService
	logger    *slog.Logger
}

// NewStudentChangeLogCleanupService constructs the cleanup service. auditRepo
// receives one data_deletions row per affected student; settings may be nil in
// tests, in which case retention resolves to the last-resort default.
func NewStudentChangeLogCleanupService(
	repo auditModels.StudentFieldEditRepository,
	auditRepo auditModels.DataDeletionRepository,
	settings config.SettingsService,
	logger *slog.Logger,
) StudentChangeLogCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &studentChangeLogCleanupService{
		repo:      repo,
		auditRepo: auditRepo,
		settings:  settings,
		logger:    logger,
	}
}

// CleanupExpiredChangeLog deletes change-history rows older than the resolved
// retention window. Writes one audit.data_deletions row per affected student
// BEFORE the delete, so any failure rolls back everything atomically inside the
// caller's transaction.
func (s *studentChangeLogCleanupService) CleanupExpiredChangeLog(ctx context.Context) (*StudentChangeLogCleanupResult, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("student change-log cleanup: no tenant in context")
	}

	start := time.Now()
	retentionDays, err := s.resolveRetentionDays(ctx)
	if err != nil {
		return nil, err
	}
	// created_at is TIMESTAMPTZ, so compare against an instant. BerlinMidnight
	// of (today - retentionDays) is the cutoff; anything older is deleted.
	cutoff := timezone.TodayDate().AddDays(-retentionDays).BerlinMidnight()

	// 1. Per-student impact, needed before the delete so audit rows can be
	//    written first.
	counts, err := s.repo.CountOlderThanByStudent(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("count expired change-log rows: %w", err)
	}

	// Nothing to do — skip the audit write and the delete entirely.
	if len(counts) == 0 {
		return &StudentChangeLogCleanupResult{
			Success:       true,
			RetentionDays: retentionDays,
			Cutoff:        cutoff,
			DurationMS:    time.Since(start).Milliseconds(),
		}, nil
	}

	// 2. One audit.data_deletions row per affected student, BEFORE the delete.
	if err := s.writeStudentAuditRows(ctx, tenantID, retentionDays, cutoff, counts); err != nil {
		return nil, fmt.Errorf("write audit rows: %w", err)
	}

	// 3. Delete the expired rows.
	deleted, err := s.repo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return nil, fmt.Errorf("delete expired change-log rows: %w", err)
	}

	result := &StudentChangeLogCleanupResult{
		Success:          true,
		EditsDeleted:     int(deleted),
		StudentsAffected: len(counts),
		RetentionDays:    retentionDays,
		Cutoff:           cutoff,
		DurationMS:       time.Since(start).Milliseconds(),
	}

	s.logger.Info("student change-log cleanup completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int("edits_deleted", result.EditsDeleted),
		slog.Int("students_affected", result.StudentsAffected),
		slog.Int("retention_days", retentionDays),
		slog.Int64("duration_ms", result.DurationMS),
	)

	return result, nil
}

// writeStudentAuditRows inserts one audit.data_deletions row per affected
// student. Called BEFORE the delete so any failure rolls back everything
// atomically (caller-supplied WithTenantTx).
func (s *studentChangeLogCleanupService) writeStudentAuditRows(
	ctx context.Context,
	tenantID int64,
	retentionDays int,
	cutoff time.Time,
	counts map[int64]int,
) error {
	if s.auditRepo == nil {
		return fmt.Errorf("audit repo not configured")
	}
	cutoffStr := cutoff.Format(time.RFC3339)
	for studentID, n := range counts {
		deletion := auditModels.NewDataDeletion(
			studentID,
			auditModels.DeletionTypeStudentChangeLogRetention,
			n,
			"system",
		)
		deletion.SetTenantID(tenantID)
		deletion.DeletionReason = "automated student change-log retention cleanup"
		deletion.SetMetadata("retention_days", retentionDays)
		deletion.SetMetadata("cutoff", cutoffStr)
		if err := s.auditRepo.Create(ctx, deletion); err != nil {
			return fmt.Errorf("audit row for student %d: %w", studentID, err)
		}
	}
	return nil
}

// resolveRetentionDays picks the tenant's retention days: tenant DB override or
// registry default. This setting has no env-var fallback, so ResolveInt already
// returns the registry default (90) when no tenant override exists — there is no
// "no override" case to distinguish, hence no HasTenantOverride check. The
// literal fallback is only reached when the settings service is not wired in
// tests. A resolution error aborts cleanup so a transient settings failure
// cannot shorten a tenant's configured retention period.
func (s *studentChangeLogCleanupService) resolveRetentionDays(ctx context.Context) (int, error) {
	if s.settings == nil {
		return studentChangeLogRetentionDefaultDays, nil
	}
	v, err := s.settings.ResolveInt(ctx, configModel.KeyGDPRStudentChangeLogRetentionDays)
	if err != nil {
		return 0, fmt.Errorf("resolve student change-log retention: %w", err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("student change-log retention must be positive, got %d", v)
	}
	return v, nil
}
