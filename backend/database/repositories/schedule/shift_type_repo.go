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

	if where, val, ok := base.TenantWhere(ctx, "shift_type"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list all shift types", Err: err}
	}
	return shiftTypes, nil
}

// List applies QueryOptions, tenant-scoped (satisfies base.Repository[T]).
func (r *ShiftTypeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*schedule.ShiftType, error) {
	var shiftTypes []*schedule.ShiftType
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&shiftTypes).
		ModelTableExpr(tableExprShiftTypesAsShiftType)

	if where, val, ok := base.TenantWhere(ctx, "shift_type"); ok {
		query = query.Where(where, val)
	}
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list shift types", Err: err}
	}
	return shiftTypes, nil
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
		return &modelBase.DatabaseError{Op: "update shift type", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "update shift type")
}
