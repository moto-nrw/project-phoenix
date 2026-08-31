package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableStaffBalanceAdjustments = "active.staff_balance_adjustments"
	// bun expands Model(&rows) columns under the struct-derived alias, so the
	// FROM must bind it (same pattern as staff_absence.go).
	tableExprStaffBalanceAdjustmentsAliased = `active.staff_balance_adjustments AS "staff_balance_adjustment"`
)

type StaffBalanceAdjustmentRepository struct {
	*base.Repository[*active.StaffBalanceAdjustment]
	db *bun.DB
}

func NewStaffBalanceAdjustmentRepository(db *bun.DB) active.StaffBalanceAdjustmentRepository {
	repo := base.NewRepository[*active.StaffBalanceAdjustment](db, tableStaffBalanceAdjustments, "StaffBalanceAdjustment")
	repo.TenantScoped = true
	return &StaffBalanceAdjustmentRepository{Repository: repo, db: db}
}

// List overrides base List to use QueryOptions (interface contract).
func (r *StaffBalanceAdjustmentRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffBalanceAdjustment, error) {
	return r.ListWithOptions(ctx, options)
}

// LockStaffBalanceWrites serializes balance-adjustment writes for one
// tenant/staff pair. Every adjustment mutation takes the same lock so no
// create/delete can commit inside ResetBalance's read-compute-insert sequence.
func (r *StaffBalanceAdjustmentRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return lockStaffBalanceWrites(ctx, r.db, staffID)
}

// GetByStaffAndDateRange returns adjustments whose effective_date lies in
// [from, to], ordered by effective_date then id.
func (r *StaffBalanceAdjustmentRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*active.StaffBalanceAdjustment, error) {
	var adjustments []*active.StaffBalanceAdjustment
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&adjustments).
		ModelTableExpr(tableExprStaffBalanceAdjustmentsAliased).
		Where(`"staff_balance_adjustment".staff_id = ?`, staffID).
		Where(`"staff_balance_adjustment".effective_date >= ?`, from).
		Where(`"staff_balance_adjustment".effective_date <= ?`, to).
		OrderExpr(`"staff_balance_adjustment".effective_date ASC, "staff_balance_adjustment".id ASC`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"staff_balance_adjustment".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get balance adjustments by staff+range", Err: base.TranslateNotFound(err)}
	}
	return adjustments, nil
}

// GetByStaffIDsAndDateRange is GetByStaffAndDateRange for many staff members in
// one round trip, keyed by staff ID. A batched IN-lookup the generic filter API
// cannot express as a single query.
func (r *StaffBalanceAdjustmentRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*active.StaffBalanceAdjustment, error) {
	result := make(map[int64][]*active.StaffBalanceAdjustment, len(staffIDs))
	if len(staffIDs) == 0 {
		// bun renders an empty IN list as invalid SQL.
		return result, nil
	}

	var adjustments []*active.StaffBalanceAdjustment
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&adjustments).
		ModelTableExpr(tableExprStaffBalanceAdjustmentsAliased).
		Where(`"staff_balance_adjustment".staff_id IN (?)`, bun.List(staffIDs)).
		Where(`"staff_balance_adjustment".effective_date >= ?`, from).
		Where(`"staff_balance_adjustment".effective_date <= ?`, to).
		OrderExpr(`"staff_balance_adjustment".staff_id ASC, "staff_balance_adjustment".effective_date ASC, "staff_balance_adjustment".id ASC`)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where(`"staff_balance_adjustment".tenant_id = ?`, tenantID)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get balance adjustments by staff IDs+range", Err: base.TranslateNotFound(err)}
	}
	for _, adjustment := range adjustments {
		result[adjustment.StaffID] = append(result[adjustment.StaffID], adjustment)
	}
	return result, nil
}
