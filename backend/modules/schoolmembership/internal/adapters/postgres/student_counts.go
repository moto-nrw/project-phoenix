package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/uptrace/bun"
)

// activelyManagedPredicate is the billing rule for "actively managed": a live
// membership in status active whose enrollment has not ended before the
// capture day (#2791). A start after the capture day does not exclude it:
// immediate activation enrolls an active child before its formal start, as in
// the current-care rule of the student directory. Its one parameter is the
// capture day.
const activelyManagedPredicate = `deleted_at IS NULL
  AND status = 'active'
  AND (enrolled_until IS NULL OR enrolled_until >= ?::date)`

// countActiveStudentsByTenantSQL is the Stichtagszahl of every school.
const countActiveStudentsByTenantSQL = `SELECT tenant_id, COUNT(*) AS count
FROM users.student_school_memberships
WHERE ` + activelyManagedPredicate + `
GROUP BY tenant_id`

// childQuotaPredicate is the Kontingentzahl rule (#3567): the actively
// managed children of the Stichtagszahl plus the live pending ones, whose
// care starts later. It extends the billing rule instead of restating it, so
// the two numbers cannot drift apart. Its three parameters are the count day.
const childQuotaPredicate = `((` + activelyManagedPredicate + `)
    OR (deleted_at IS NULL
      AND status = 'pending'
      AND (enrolled_until IS NULL OR enrolled_until >= ?::date)
      AND enrolled_from > ?::date))`

// countChildQuotaSQL is the Kontingentzahl of one school.
const countChildQuotaSQL = `SELECT COUNT(*)
FROM users.student_school_memberships
WHERE tenant_id = ?
  AND ` + childQuotaPredicate

// countChildQuotaByTenantSQL is the Kontingentzahl of every school.
const countChildQuotaByTenantSQL = `SELECT tenant_id, COUNT(*) AS count
FROM users.student_school_memberships
WHERE ` + childQuotaPredicate + `
GROUP BY tenant_id`

// childQuotaLockKey is the advisory-lock namespace of the counting writes
// ("kont"); the second key is the tenant.
const childQuotaLockKey = int32(0x6b6f6e74)

// BerlinToday is the calendar day (YYYY-MM-DD) the Kontingentzahl is counted
// on: the school's day, not the database server's.
func BerlinToday() string {
	return calendar.DateFromTime(time.Now()).String()
}

// CountActiveStudentsByTenant counts the active students of every school the
// connection can see.
func CountActiveStudentsByTenant(ctx context.Context, db bun.IDB, capturedAt time.Time) (map[int64]int, error) {
	captureDate := calendar.DateFromTime(capturedAt)
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		Count    int   `bun:"count"`
	}
	if err := db.NewRaw(countActiveStudentsByTenantSQL, captureDate).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school membership postgres: count active students by tenant: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.TenantID] = row.Count
	}
	return counts, nil
}

// CountChildQuotaByTenant counts the Kontingentzahl of every school the
// connection can see on the given calendar day (YYYY-MM-DD).
func CountChildQuotaByTenant(ctx context.Context, db bun.IDB, on string) (map[int64]int, error) {
	day, err := calendar.ParseDate(on)
	if err != nil {
		return nil, fmt.Errorf("school membership postgres: parse count day: %w", err)
	}
	var rows []struct {
		TenantID int64 `bun:"tenant_id"`
		Count    int   `bun:"count"`
	}
	if err := db.NewRaw(countChildQuotaByTenantSQL, day, day, day).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school membership postgres: count child quota by tenant: %w", err)
	}
	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.TenantID] = row.Count
	}
	return counts, nil
}

// LockChildQuota takes the tenant's exclusive counting lock. A shared lock
// would not do: two shared holders never wait for each other, so two
// parallel enrollments could both count below the Kinderkontingent.
func (s *Store) LockChildQuota(ctx context.Context) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if tenantID <= 0 || tenantID > 0x7fffffff {
		return domain.OperationStats{}, errors.New("school membership: valid tenant is required")
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewRaw("SELECT pg_advisory_xact_lock(?, ?)", childQuotaLockKey, int32(tenantID)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("school membership postgres: lock child quota: %w", err)
	}
	return stats, nil
}

// CountChildQuota counts the Kontingentzahl of the tenant in context on the
// given calendar day.
func (s *Store) CountChildQuota(ctx context.Context, on string) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	day, err := calendar.ParseDate(on)
	if err != nil {
		return 0, domain.OperationStats{}, fmt.Errorf("school membership postgres: parse count day: %w", err)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	var count int
	err = db.NewRaw(countChildQuotaSQL, tenantID, day, day, day).Scan(ctx, &count)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("school membership postgres: count child quota: %w", err)
	}
	return count, stats, nil
}
