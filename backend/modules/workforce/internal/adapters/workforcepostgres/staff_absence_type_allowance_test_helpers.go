//go:build testhooks

package workforce

import (
	"context"
	"fmt"
	"time"

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

// Database resolves the connection and the tenant of the current request,
// the same required-school runtime the Workforce store reads through.
type Database func(context.Context) (bun.IDB, int64, error)

// StaffAbsenceTypeAllowance is one person's yearly claim for one
// school-defined absence type, as the isolation test reads it back.
type StaffAbsenceTypeAllowance struct {
	bun.BaseModel `bun:"table:active.staff_absence_type_allowances,alias:staff_absence_type_allowance"`

	ID            int64     `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	StaffID       int64     `bun:"staff_id,notnull"`
	AbsenceTypeID int64     `bun:"absence_type_id,notnull"`
	Year          int       `bun:"year,notnull"`
	EntitledDays  float64   `bun:"entitled_days,notnull"`
}

// StaffAbsenceTypeAllowanceChange is one entry of the claim's append-only
// reason trail, as the isolation test reads it back.
type StaffAbsenceTypeAllowanceChange struct {
	bun.BaseModel `bun:"table:active.staff_absence_type_allowance_changes,alias:staff_absence_type_allowance_change"`

	ID              int64     `bun:"id,pk,autoincrement"`
	CreatedAt       time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt       time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID        int64     `bun:"tenant_id,notnull"`
	StaffID         int64     `bun:"staff_id,notnull"`
	AbsenceTypeID   int64     `bun:"absence_type_id,notnull"`
	Year            int       `bun:"year,notnull"`
	OldEntitledDays *float64  `bun:"old_entitled_days"`
	NewEntitledDays float64   `bun:"new_entitled_days,notnull"`
	Reason          string    `bun:"reason,notnull"`
	ChangedBy       int64     `bun:"changed_by,notnull"`
}

type StaffAbsenceTypeAllowanceRepository struct{ database Database }

func NewStaffAbsenceTypeAllowanceRepository(database Database) *StaffAbsenceTypeAllowanceRepository {
	if database == nil {
		panic("staff absence type allowance repository: database runtime is required")
	}
	return &StaffAbsenceTypeAllowanceRepository{database: database}
}

// List returns every allowance row the bound tenant can see. A bound tenant
// adds the defense-in-depth tenant_id predicate on top of row-level security.
func (r *StaffAbsenceTypeAllowanceRepository) List(ctx context.Context) ([]*StaffAbsenceTypeAllowance, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]*StaffAbsenceTypeAllowance, 0)
	query := db.NewSelect().Model(&rows).ModelTableExpr(tableStaffAbsenceTypeAllowances + ` AS "staff_absence_type_allowance"`)
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant is required")
	}
	query = query.Where("tenant_id = ?", tenantID)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list staff absence type allowances: %w", err)
	}
	return rows, nil
}

type StaffAbsenceTypeAllowanceChangeRepository struct{ database Database }

func NewStaffAbsenceTypeAllowanceChangeRepository(database Database) *StaffAbsenceTypeAllowanceChangeRepository {
	if database == nil {
		panic("staff absence type allowance change repository: database runtime is required")
	}
	return &StaffAbsenceTypeAllowanceChangeRepository{database: database}
}

// List returns every allowance change row the bound tenant can see. A bound
// tenant adds the defense-in-depth tenant_id predicate on top of row-level
// security, as for the allowances.
func (r *StaffAbsenceTypeAllowanceChangeRepository) List(ctx context.Context) ([]*StaffAbsenceTypeAllowanceChange, error) {
	db, tenantID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]*StaffAbsenceTypeAllowanceChange, 0)
	query := db.NewSelect().Model(&rows).ModelTableExpr(tableStaffAbsenceTypeAllowanceChange + ` AS "staff_absence_type_allowance_change"`)
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant is required")
	}
	query = query.Where("tenant_id = ?", tenantID)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list staff absence type allowance changes: %w", err)
	}
	return rows, nil
}
