package compose

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StudentChangeLogCleanupResult summarises one cleanup run for a single tenant.
type StudentChangeLogCleanupResult struct {
	Success          bool
	EditsDeleted     int
	StudentsAffected int
	RetentionDays    int
	Cutoff           time.Time
	DurationMS       int64
}

// StudentChangeLogStore is the per-child change history the sweep trims
// (audit.student_field_edits). The composition root binds the retained
// repository.
type StudentChangeLogStore interface {
	// CountOlderThanByStudent returns, per student, the number of rows created
	// strictly before cutoff.
	CountOlderThanByStudent(ctx context.Context, cutoff time.Time) (map[int64]int, error)
	// DeleteOlderThan removes every row created strictly before cutoff and
	// returns how many it removed.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// StudentChangeLogDeletion is one child's share of a retention run, as the
// GDPR deletion trail records it.
type StudentChangeLogDeletion struct {
	TenantID      int64
	StudentID     int64
	EditsDeleted  int
	RetentionDays int
	// Cutoff is the run's cutoff instant in RFC 3339.
	Cutoff string
}

// StudentChangeLogDeletionLog appends one entry to the GDPR deletion trail
// (audit.data_deletions). The composition root binds it; Audit Platform owns
// the trail.
type StudentChangeLogDeletionLog func(ctx context.Context, deletion StudentChangeLogDeletion) error

// StudentChangeLogRetention resolves the tenant's retention days
// (gdpr.student_change_log_retention_days): the tenant override or the
// registry default, without an env fallback.
type StudentChangeLogRetention func(ctx context.Context) (int, error)

// StudentChangeLogCleanupService drives GDPR retention cleanup for the
// per-child change history (audit.student_field_edits, issue #1455). It left
// services/users in #3753. All methods assume tenant context has been
// established by the caller (WithTenantTx) — the scheduler does this per
// tenant.
type StudentChangeLogCleanupService interface {
	CleanupExpiredChangeLog(ctx context.Context) (*StudentChangeLogCleanupResult, error)
}

type studentChangeLogCleanupService struct {
	store     StudentChangeLogStore
	deletions StudentChangeLogDeletionLog
	retention StudentChangeLogRetention
	logger    *slog.Logger
}

// NewStudentChangeLogCleanupService constructs the cleanup service. deletions
// receives one entry per affected student. Cleanup fails closed when the
// retention setting is not wired, so it never applies an unverified retention
// policy.
func NewStudentChangeLogCleanupService(
	store StudentChangeLogStore,
	deletions StudentChangeLogDeletionLog,
	retention StudentChangeLogRetention,
	logger *slog.Logger,
) StudentChangeLogCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &studentChangeLogCleanupService{
		store:     store,
		deletions: deletions,
		retention: retention,
		logger:    logger,
	}
}

// CleanupExpiredChangeLog deletes change-history rows older than the resolved
// retention window. Writes one deletion-trail entry per affected student
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
	cutoff := calendar.TodayDate().AddDays(-retentionDays).BerlinMidnight()

	// 1. Per-student impact, needed before the delete so audit rows can be
	//    written first.
	counts, err := s.store.CountOlderThanByStudent(ctx, cutoff)
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

	// 2. One deletion-trail entry per affected student, BEFORE the delete.
	if err := s.writeStudentAuditRows(ctx, tenantID, retentionDays, cutoff, counts); err != nil {
		return nil, fmt.Errorf("write audit rows: %w", err)
	}

	// 3. Delete the expired rows.
	deleted, err := s.store.DeleteOlderThan(ctx, cutoff)
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

	s.logCompleted(tenantID, result)
	return result, nil
}

func (s *studentChangeLogCleanupService) logCompleted(tenantID int64, result *StudentChangeLogCleanupResult) {
	s.logger.Info("student change-log cleanup completed",
		slog.Int64("tenant_id", tenantID),
		slog.Int("edits_deleted", result.EditsDeleted),
		slog.Int("students_affected", result.StudentsAffected),
		slog.Int("retention_days", result.RetentionDays),
		slog.Int64("duration_ms", result.DurationMS),
	)
}

// writeStudentAuditRows appends one deletion-trail entry per affected student.
// Called BEFORE the delete so any failure rolls back everything atomically
// (caller-supplied WithTenantTx).
func (s *studentChangeLogCleanupService) writeStudentAuditRows(
	ctx context.Context,
	tenantID int64,
	retentionDays int,
	cutoff time.Time,
	counts map[int64]int,
) error {
	if s.deletions == nil {
		return fmt.Errorf("audit repo not configured")
	}
	cutoffStr := cutoff.Format(time.RFC3339)
	for studentID, n := range counts {
		if err := s.deletions(ctx, StudentChangeLogDeletion{
			TenantID:      tenantID,
			StudentID:     studentID,
			EditsDeleted:  n,
			RetentionDays: retentionDays,
			Cutoff:        cutoffStr,
		}); err != nil {
			return fmt.Errorf("audit row for student %d: %w", studentID, err)
		}
	}
	return nil
}

// resolveRetentionDays picks the tenant's retention days: tenant DB override or
// registry default. This setting has no env-var fallback, so the resolver
// already returns the registry default (90) when no tenant override exists —
// there is no "no override" case to distinguish. Missing settings wiring or a
// resolution error aborts cleanup so no unverified retention period can delete
// tenant data.
func (s *studentChangeLogCleanupService) resolveRetentionDays(ctx context.Context) (int, error) {
	if s.retention == nil {
		return 0, fmt.Errorf("resolve student change-log retention: settings service not configured")
	}
	v, err := s.retention(ctx)
	if err != nil {
		return 0, fmt.Errorf("resolve student change-log retention: %w", err)
	}
	if v <= 0 {
		return 0, fmt.Errorf("student change-log retention must be positive, got %d", v)
	}
	return v, nil
}
