package workforce

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/uptrace/bun"
)

// The allowance rows are owned and written by the Workforce module. These
// retained repositories read them back per tenant, which is how the RLS
// isolation test verifies that neither the claim nor its audit crosses the
// tenant boundary.
const (
	tableStaffAbsenceTypeAllowances      = "active.staff_absence_type_allowances"
	tableStaffAbsenceTypeAllowanceChange = "active.staff_absence_type_allowance_changes"
)

type StaffAbsenceTypeAllowanceRepository struct {
	*base.Repository[*active.StaffAbsenceTypeAllowance]
	db *bun.DB
}

func NewStaffAbsenceTypeAllowanceRepository(db *bun.DB) active.StaffAbsenceTypeAllowanceRepository {
	repo := base.NewRepository[*active.StaffAbsenceTypeAllowance](db, tableStaffAbsenceTypeAllowances, "StaffAbsenceTypeAllowance")
	repo.TenantScoped = true
	return &StaffAbsenceTypeAllowanceRepository{Repository: repo, db: db}
}

func (r *StaffAbsenceTypeAllowanceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffAbsenceTypeAllowance, error) {
	return r.ListWithOptions(ctx, options)
}

type StaffAbsenceTypeAllowanceChangeRepository struct {
	*base.Repository[*active.StaffAbsenceTypeAllowanceChange]
}

func NewStaffAbsenceTypeAllowanceChangeRepository(db *bun.DB) active.StaffAbsenceTypeAllowanceChangeRepository {
	repo := base.NewRepository[*active.StaffAbsenceTypeAllowanceChange](db, tableStaffAbsenceTypeAllowanceChange, "StaffAbsenceTypeAllowanceChange")
	repo.TenantScoped = true
	return &StaffAbsenceTypeAllowanceChangeRepository{Repository: repo}
}

func (r *StaffAbsenceTypeAllowanceChangeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffAbsenceTypeAllowanceChange, error) {
	return r.ListWithOptions(ctx, options)
}
