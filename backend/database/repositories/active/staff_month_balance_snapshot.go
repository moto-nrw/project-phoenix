package active

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableStaffMonthBalanceSnapshots = "active.staff_month_balance_snapshots"
	// bun expands Model(&rows) columns under the struct-derived alias, so the
	// FROM must bind it (same pattern as staff_balance_adjustment.go).
	tableExprStaffMonthBalanceSnapshotsAliased = `active.staff_month_balance_snapshots AS "staff_month_balance_snapshot"`
	// monthOrdinalExpr collapses (year, month) into one sortable integer so
	// "at or before this month" is a single comparison instead of an
	// OR-chained year/month predicate.
	monthOrdinalExpr = `("staff_month_balance_snapshot".year * 12 + "staff_month_balance_snapshot".month)`
)

type StaffMonthBalanceSnapshotRepository struct {
	*base.Repository[*active.StaffMonthBalanceSnapshot]
	db *bun.DB
}

func NewStaffMonthBalanceSnapshotRepository(db *bun.DB) active.StaffMonthBalanceSnapshotRepository {
	repo := base.NewRepository[*active.StaffMonthBalanceSnapshot](db, tableStaffMonthBalanceSnapshots, "StaffMonthBalanceSnapshot")
	repo.TenantScoped = true
	return &StaffMonthBalanceSnapshotRepository{Repository: repo, db: db}
}

// List overrides base List to use QueryOptions (interface contract).
func (r *StaffMonthBalanceSnapshotRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffMonthBalanceSnapshot, error) {
	return r.ListWithOptions(ctx, options)
}

// LockStaffBalanceWrites takes the same advisory lock as the adjustment ledger,
// so closing a month and booking a payout for the same staff member serialize.
func (r *StaffMonthBalanceSnapshotRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return lockStaffBalanceWrites(ctx, r.db, staffID)
}

// monthOrdinal is the Go side of monthOrdinalExpr.
func monthOrdinal(year, month int) int {
	return year*12 + month
}

// GetLatestClosedThrough returns the newest active snapshot at or before
// (year, month), or nil when the staff member has none.
func (r *StaffMonthBalanceSnapshotRepository) GetLatestClosedThrough(ctx context.Context, staffID int64, year, month int) (*active.StaffMonthBalanceSnapshot, error) {
	snapshot := new(active.StaffMonthBalanceSnapshot)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(snapshot).
		ModelTableExpr(tableExprStaffMonthBalanceSnapshotsAliased).
		Where(`"staff_month_balance_snapshot".staff_id = ?`, staffID).
		Where(`"staff_month_balance_snapshot".reopened_at IS NULL`).
		Where(monthOrdinalExpr+` <= ?`, monthOrdinal(year, month)).
		OrderExpr(monthOrdinalExpr + ` DESC`).
		Limit(1)

	query = r.withTenantFilter(ctx, query)

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "get latest month balance snapshot", Err: err}
	}
	return snapshot, nil
}

// GetByMonth returns the active snapshots of one month across the tenant,
// ordered by staff ID so callers see a stable list.
func (r *StaffMonthBalanceSnapshotRepository) GetByMonth(ctx context.Context, year, month int) ([]*active.StaffMonthBalanceSnapshot, error) {
	var snapshots []*active.StaffMonthBalanceSnapshot
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&snapshots).
		ModelTableExpr(tableExprStaffMonthBalanceSnapshotsAliased).
		Where(`"staff_month_balance_snapshot".year = ?`, year).
		Where(`"staff_month_balance_snapshot".month = ?`, month).
		Where(`"staff_month_balance_snapshot".reopened_at IS NULL`).
		OrderExpr(`"staff_month_balance_snapshot".staff_id ASC`)

	query = r.withTenantFilter(ctx, query)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get month balance snapshots by month", Err: err}
	}
	return snapshots, nil
}

// withTenantFilter adds the explicit tenant predicate on top of RLS. The
// staff_id list is never authorization: a missing tenant_id filter would turn
// any future caller into a cross-tenant leak.
func (r *StaffMonthBalanceSnapshotRepository) withTenantFilter(ctx context.Context, query *bun.SelectQuery) *bun.SelectQuery {
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		return query.Where(`"staff_month_balance_snapshot".tenant_id = ?`, tenantID)
	}
	return query
}
