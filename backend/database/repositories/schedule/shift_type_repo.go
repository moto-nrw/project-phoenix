package schedule

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableScheduleShiftTypes        = "schedule.shift_types"
	tableExprShiftTypesAsShiftType = `schedule.shift_types AS "shift_type"`
)

// ShiftTypeRepository implements schedule.ShiftTypeRepository.
type ShiftTypeRepository struct {
	*base.Repository[*schedule.ShiftType]
	db *bun.DB
}

// NewShiftTypeRepository creates a new shift type repository.
func NewShiftTypeRepository(db *bun.DB) schedule.ShiftTypeRepository {
	repo := base.NewRepository[*schedule.ShiftType](db, tableScheduleShiftTypes, "ShiftType")
	repo.TenantScoped = true
	return &ShiftTypeRepository{Repository: repo, db: db}
}

// ListAll returns all shift types for the current tenant, ordered by name.
func (r *ShiftTypeRepository) ListAll(ctx context.Context) ([]*schedule.ShiftType, error) {
	var shiftTypes []*schedule.ShiftType
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shiftTypes).
		ModelTableExpr(tableExprShiftTypesAsShiftType).
		OrderExpr(`"shift_type".name ASC`)

	query = base.WithTenantFilter(ctx, query, "shift_type")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list all shift types", Err: base.TranslateNotFound(err)}
	}
	return shiftTypes, nil
}

// List applies QueryOptions, tenant-scoped (satisfies base.Repository[T]).
func (r *ShiftTypeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.ShiftType, error) {
	return r.ListWithOptions(ctx, options)
}

// Create validates before delegating to the generic insert.
func (r *ShiftTypeRepository) Create(ctx context.Context, shiftType *schedule.ShiftType) error {
	if shiftType == nil {
		return fmt.Errorf("shift type cannot be nil")
	}
	if err := shiftType.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, shiftType)
}

// CreateIfAbsent inserts the shift type unless one with the same
// (tenant_id, LOWER(name)) already exists (uniq_shift_types_tenant_name,
// migration 1.15.170). It returns true when a new row was inserted and false
// when a shift type with that name was already present.
//
// Why a custom method instead of the generic Create: default seeding
// (CreateDefaultShiftTypes) must be idempotent under concurrent requests. A
// read-then-insert sequence can race (two parallel seeds both snapshot "no
// such default" and both insert — one fails with a unique violation that,
// because the seed runs inside the request's TenantTxMiddleware transaction,
// aborts the whole transaction and makes every later statement fail with
// "current transaction is aborted"). ON CONFLICT DO NOTHING is the only
// race-free shape: exactly one insert wins, the loser is a clean no-op, and
// the transaction stays usable. Mirrors base.Create's validate +
// tenant-autoset + ModelTableExpr insert, adding only the conflict clause.
func (r *ShiftTypeRepository) CreateIfAbsent(ctx context.Context, shiftType *schedule.ShiftType) (bool, error) {
	if shiftType == nil {
		return false, fmt.Errorf("shift type cannot be nil")
	}
	if err := shiftType.Validate(); err != nil {
		return false, err
	}
	base.EnsureTenantID(ctx, shiftType)

	res, err := base.GetDB(ctx, r.db).NewInsert().
		Model(shiftType).
		ModelTableExpr(tableScheduleShiftTypes).
		On("CONFLICT (tenant_id, LOWER(name)) DO NOTHING").
		Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "create shift type if absent", Err: base.TranslateNotFound(err)}
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "create shift type if absent (rows affected)", Err: base.TranslateNotFound(err)}
	}
	return affected == 1, nil
}

// Update validates and persists changes, tenant-scoped.
func (r *ShiftTypeRepository) Update(ctx context.Context, shiftType *schedule.ShiftType) error {
	if shiftType == nil {
		return fmt.Errorf("shift type cannot be nil")
	}
	if err := shiftType.Validate(); err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(shiftType).
		Column("name", "color", "description", "is_active").
		Where("id = ?", shiftType.ID).
		ModelTableExpr(tableScheduleShiftTypes)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update shift type", Err: base.TranslateNotFound(err)}
	}
	return base.AssertRowsAffected(result, 1, "update shift type")
}

// Delete removes the shift type by ID, tenant-scoped. Overrides the generic
// base delete on purpose: this file's write paths (Create/CreateIfAbsent/
// Update/Delete) all pin the unaliased schedule.shift_types table expression
// explicitly, so the predicates here stay unaliased to match. Shifts
// referencing the type keep their row; the FK's ON DELETE SET NULL
// (shift_type_id) clears only the reference.
func (r *ShiftTypeRepository) Delete(ctx context.Context, id any) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*schedule.ShiftType)(nil)).
		ModelTableExpr(tableScheduleShiftTypes).
		Where("id = ?", id)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete shift type", Err: base.TranslateNotFound(err)}
	}
	return nil
}
