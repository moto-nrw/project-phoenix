package active

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	tableStaffAbsenceTypes             = "active.staff_absence_types"
	tableExprStaffAbsenceTypesAsAsType = `active.staff_absence_types AS "staff_absence_type"`
)

// StaffAbsenceTypeRepository implements active.StaffAbsenceTypeRepository.
type StaffAbsenceTypeRepository struct {
	*base.Repository[*active.StaffAbsenceType]
	db *bun.DB
}

// NewStaffAbsenceTypeRepository creates a new staff absence type repository.
func NewStaffAbsenceTypeRepository(db *bun.DB) active.StaffAbsenceTypeRepository {
	repo := base.NewRepository[*active.StaffAbsenceType](db, tableStaffAbsenceTypes, "StaffAbsenceType")
	repo.TenantScoped = true
	return &StaffAbsenceTypeRepository{Repository: repo, db: db}
}

// ListAll returns all absence types for the current tenant, ordered by name.
// Inactive entries are included on purpose: historical absences must still
// resolve to the name they were filed under.
func (r *StaffAbsenceTypeRepository) ListAll(ctx context.Context) ([]*active.StaffAbsenceType, error) {
	var types []*active.StaffAbsenceType
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&types).
		ModelTableExpr(tableExprStaffAbsenceTypesAsAsType).
		OrderExpr(`"staff_absence_type".name ASC`)

	query = base.WithTenantFilter(ctx, query, "staff_absence_type")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list all staff absence types", Err: err}
	}
	return types, nil
}

// IsInUse reports whether a staff absence in the current tenant references
// this art. A used art cannot be renamed, because old entries must retain the
// name under which they were recorded.
func (r *StaffAbsenceTypeRepository) IsInUse(ctx context.Context, id int64) (bool, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`active.staff_absences AS "staff_absence"`).
		Where(`"staff_absence".absence_type_id = ?`, id)
	query = base.WithTenantFilter(ctx, query, "staff_absence")

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check staff absence type usage", Err: err}
	}
	return exists, nil
}

// List applies QueryOptions, tenant-scoped (satisfies base.Repository[T]).
func (r *StaffAbsenceTypeRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.StaffAbsenceType, error) {
	return r.ListWithOptions(ctx, options)
}

// Create validates before delegating to the generic insert.
func (r *StaffAbsenceTypeRepository) Create(ctx context.Context, absenceType *active.StaffAbsenceType) error {
	if absenceType == nil {
		return fmt.Errorf("absence type cannot be nil")
	}
	if err := absenceType.Validate(); err != nil {
		return err
	}
	return r.Repository.Create(ctx, absenceType)
}

// Update persists name and active flag, tenant-scoped. base_type is
// deliberately not in the column list: an art's arithmetic is fixed at
// creation, so a rename can never move existing absences into another
// calculation bucket.
func (r *StaffAbsenceTypeRepository) Update(ctx context.Context, absenceType *active.StaffAbsenceType) error {
	if absenceType == nil {
		return fmt.Errorf("absence type cannot be nil")
	}
	if err := absenceType.Validate(); err != nil {
		return err
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(absenceType).
		Column("name", "is_active").
		Where("id = ?", absenceType.ID).
		ModelTableExpr(tableStaffAbsenceTypes)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update staff absence type", Err: err}
	}
	return base.AssertRowsAffected(result, 1, "update staff absence type")
}
