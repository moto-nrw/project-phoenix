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
