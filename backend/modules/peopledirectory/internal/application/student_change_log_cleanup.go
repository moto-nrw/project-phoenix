package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewStudentChangeLogCleanupService constructs the cleanup service. tenants
// names the caller's tenant; deletions receives one entry per affected
// student. Cleanup fails closed when the retention setting is not wired, so it
// never applies an unverified retention policy.
func NewStudentChangeLogCleanupService(
	tenants internalports.Transaction,
	store ports.StudentChangeLogStore,
	deletions ports.StudentChangeLogDeletionLog,
	retention ports.StudentChangeLogRetention,
	logger *slog.Logger,
) ports.StudentChangeLogCleanupService {
	if logger == nil {
		logger = slog.Default()
	}
	return &studentChangeLogCleanupService{
		tenants:   tenants,
		store:     store,
		deletions: deletions,
		retention: retention,
		logger:    logger,
	}
}

type studentChangeLogCleanupService struct {
	tenants   internalports.Transaction
	store     ports.StudentChangeLogStore
	deletions ports.StudentChangeLogDeletionLog
	retention ports.StudentChangeLogRetention
	logger    *slog.Logger
}

// CleanupExpiredChangeLog deletes change-history rows older than the resolved
// retention window. Writes one deletion-trail entry per affected student
// BEFORE the delete, so any failure rolls back everything atomically inside the
// caller's transaction.
func (s *studentChangeLogCleanupService) CleanupExpiredChangeLog(ctx context.Context) (*ports.StudentChangeLogCleanupResult, error) {
	tenantID := s.tenants.TenantID(ctx)
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
		return &ports.StudentChangeLogCleanupResult{
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

	result := &ports.StudentChangeLogCleanupResult{
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

func (s *studentChangeLogCleanupService) logCompleted(tenantID int64, result *ports.StudentChangeLogCleanupResult) {
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
		if err := s.deletions(ctx, ports.StudentChangeLogDeletion{
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
