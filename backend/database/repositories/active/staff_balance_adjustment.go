package active

import (
	"context"
	"errors"
	"fmt"

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
// tenant/staff pair. ResetBalance holds it through its read-compute-insert
// sequence so two concurrent resets cannot both read the same balance.
func (r *StaffBalanceAdjustmentRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("tenant id is required")
	}
	key := fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID)
	if err := base.AcquireXactLock(ctx, r.db, key); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
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
		return nil, &modelBase.DatabaseError{Op: "get balance adjustments by staff+range", Err: err}
	}
	return adjustments, nil
}
