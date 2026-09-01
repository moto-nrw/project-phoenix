package workforce

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

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

func (r *StaffAbsenceTypeAllowanceRepository) Upsert(ctx context.Context, allowance *active.StaffAbsenceTypeAllowance) error {
	if allowance == nil {
		return fmt.Errorf("allowance cannot be nil")
	}
	if err := allowance.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, allowance)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(allowance).
		ModelTableExpr(tableStaffAbsenceTypeAllowances).
		On("CONFLICT (tenant_id, staff_id, absence_type_id, year) DO UPDATE").
		Set("entitled_days = EXCLUDED.entitled_days").
		Set("updated_at = CURRENT_TIMESTAMP").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert staff absence type allowance", Err: base.TranslateNotFound(err)}
	}
	return nil
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

func (r *StaffAbsenceTypeAllowanceChangeRepository) Create(ctx context.Context, change *active.StaffAbsenceTypeAllowanceChange) error {
	if change == nil {
		return fmt.Errorf("allowance change cannot be nil")
	}
	if err := change.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, change)
}
