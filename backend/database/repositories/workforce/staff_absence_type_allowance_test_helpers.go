package workforce

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
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

// StaffAbsenceTypeAllowance is one person's yearly claim for one
// school-defined absence type, as the isolation test reads it back.
type StaffAbsenceTypeAllowance struct {
	modelBase.Model `bun:"schema:active,table:staff_absence_type_allowances"`
	modelBase.TenantModel
	StaffID       int64   `bun:"staff_id,notnull"`
	AbsenceTypeID int64   `bun:"absence_type_id,notnull"`
	Year          int     `bun:"year,notnull"`
	EntitledDays  float64 `bun:"entitled_days,notnull"`
}

// StaffAbsenceTypeAllowanceChange is one entry of the claim's append-only
// reason trail, as the isolation test reads it back.
type StaffAbsenceTypeAllowanceChange struct {
	modelBase.Model `bun:"schema:active,table:staff_absence_type_allowance_changes"`
	modelBase.TenantModel
	StaffID         int64    `bun:"staff_id,notnull"`
	AbsenceTypeID   int64    `bun:"absence_type_id,notnull"`
	Year            int      `bun:"year,notnull"`
	OldEntitledDays *float64 `bun:"old_entitled_days"`
	NewEntitledDays float64  `bun:"new_entitled_days,notnull"`
	Reason          string   `bun:"reason,notnull"`
	ChangedBy       int64    `bun:"changed_by,notnull"`
}

type StaffAbsenceTypeAllowanceRepository struct {
	*base.Repository[*StaffAbsenceTypeAllowance]
}

func NewStaffAbsenceTypeAllowanceRepository(db *bun.DB) *StaffAbsenceTypeAllowanceRepository {
	repo := base.NewRepository[*StaffAbsenceTypeAllowance](db, tableStaffAbsenceTypeAllowances, "StaffAbsenceTypeAllowance")
	repo.TenantScoped = true
	return &StaffAbsenceTypeAllowanceRepository{Repository: repo}
}

func (r *StaffAbsenceTypeAllowanceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*StaffAbsenceTypeAllowance, error) {
	return r.ListWithOptions(ctx, options)
}

type StaffAbsenceTypeAllowanceChangeRepository struct {
	*base.Repository[*StaffAbsenceTypeAllowanceChange]
}

func NewStaffAbsenceTypeAllowanceChangeRepository(db *bun.DB) *StaffAbsenceTypeAllowanceChangeRepository {
	repo := base.NewRepository[*StaffAbsenceTypeAllowanceChange](db, tableStaffAbsenceTypeAllowanceChange, "StaffAbsenceTypeAllowanceChange")
	repo.TenantScoped = true
	return &StaffAbsenceTypeAllowanceChangeRepository{Repository: repo}
}

func (r *StaffAbsenceTypeAllowanceChangeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*StaffAbsenceTypeAllowanceChange, error) {
	return r.ListWithOptions(ctx, options)
}
