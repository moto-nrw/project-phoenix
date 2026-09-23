package scheduler

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// dbTenantDirectory lists the seeded schools straight from platform.schools
// so the wired scheduler tests run against the real tenant fixtures.
// Callers pass the pool from testpkg.SetupTestDB.
type dbTenantDirectory struct{ db *bun.DB }

func (d dbTenantDirectory) ListActiveTenantIDs(ctx context.Context) ([]int64, error) {
	return d.list(ctx, true)
}

func (d dbTenantDirectory) ListNonDeletedTenantIDs(ctx context.Context) ([]int64, error) {
	return d.list(ctx, false)
}

// RecordDueBillingKeyDates captures nothing: the wired scheduler tests cover
// the tenant loops, and the billing capture has its own owner tests.
func (d dbTenantDirectory) RecordDueBillingKeyDates(context.Context, time.Time) (int, error) {
	return 0, nil
}

func (d dbTenantDirectory) list(ctx context.Context, activeOnly bool) ([]int64, error) {
	ids := []int64{}
	query := d.db.NewSelect().
		TableExpr(`platform.schools AS "school"`).
		Column("school.id").
		Where(`"school".deleted_at IS NULL`).
		OrderExpr(`"school".name ASC`)
	if activeOnly {
		query = query.Where(`"school".active = TRUE`)
	}
	err := query.Scan(ctx, &ids)
	return ids, err
}
