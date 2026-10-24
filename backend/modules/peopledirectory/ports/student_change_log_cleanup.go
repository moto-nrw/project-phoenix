package ports

import (
	"context"
	"time"
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
