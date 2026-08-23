package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const enrollmentOfferingAdjustmentTableExpr = `audit.enrollment_offering_adjustments AS "enrollment_offering_adjustment"`

type enrollmentOfferingAdjustmentRepository struct {
	*base.Repository[*audit.EnrollmentOfferingAdjustment]
	db *bun.DB
}

func NewEnrollmentOfferingAdjustmentRepository(db *bun.DB) audit.EnrollmentOfferingAdjustmentRepository {
	repo := base.NewRepository[*audit.EnrollmentOfferingAdjustment](db, "audit.enrollment_offering_adjustments", "EnrollmentOfferingAdjustment")
	repo.TenantScoped = true
	return &enrollmentOfferingAdjustmentRepository{Repository: repo, db: db}
}

func (r *enrollmentOfferingAdjustmentRepository) ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*audit.EnrollmentOfferingAdjustment, error) {
	if requestChildID <= 0 {
		return nil, fmt.Errorf("request_child_id is required")
	}
	var rows []*audit.EnrollmentOfferingAdjustment
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(enrollmentOfferingAdjustmentTableExpr).
		Where(`"enrollment_offering_adjustment".request_child_id = ?`, requestChildID).
		OrderExpr(`"enrollment_offering_adjustment".changed_at DESC, "enrollment_offering_adjustment".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list enrollment offering adjustments", Err: err}
	}
	return rows, nil
}

// ListDirectForTenant returns the tenant's direct corrections newest first.
// Request-applied rows are excluded: the central history already shows those as
// the decided request they belong to (#2436).
func (r *enrollmentOfferingAdjustmentRepository) ListDirectForTenant(
	ctx context.Context,
	filters modelBase.RequestQueueFilters,
) ([]*audit.EnrollmentOfferingAdjustment, error) {
	if filters.Limit <= 0 {
		return []*audit.EnrollmentOfferingAdjustment{}, nil
	}
	var rows []*audit.EnrollmentOfferingAdjustment
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(enrollmentOfferingAdjustmentTableExpr).
		Where(`"enrollment_offering_adjustment".source = ?`, audit.OfferingAdjustmentSourceDirect)
	query = base.WithTenantFilter(ctx, query, "enrollment_offering_adjustment")
	query = base.ApplyRequestQueueFilters(query, "enrollment_offering_adjustment", "changed_at", filters)

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list direct enrollment offering adjustments", Err: err}
	}
	return rows, nil
}
