package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

const enrollmentOfferingAdjustmentTableExpr = `audit.enrollment_offering_adjustments AS "enrollment_offering_adjustment"`

type enrollmentOfferingAdjustmentRepository struct {
	runtime Runtime
}

func NewEnrollmentOfferingAdjustmentRepository(runtime Runtime) audit.EnrollmentOfferingAdjustmentRepository {
	return &enrollmentOfferingAdjustmentRepository{runtime: requireRuntime(runtime)}
}

func (r *enrollmentOfferingAdjustmentRepository) Create(ctx context.Context, entry *audit.EnrollmentOfferingAdjustment) error {
	return NewAppender(r.runtime).Append(ctx, entry)
}

func (r *enrollmentOfferingAdjustmentRepository) CountForDeletion(
	ctx context.Context,
	requestID int64,
	requestChildID *int64,
) (int, error) {
	if requestID <= 0 {
		return 0, fmt.Errorf("request ID must be positive")
	}
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return 0, fmt.Errorf("tenant context is required")
	}
	query := runtimeDB(ctx, r.runtime).NewSelect().
		TableExpr(enrollmentOfferingAdjustmentTableExpr).
		ColumnExpr(`COUNT(*)`).
		Where(`"enrollment_offering_adjustment".tenant_id = ?`, tenantID).
		Where(`"enrollment_offering_adjustment".request_id = ?`, requestID)
	if requestChildID != nil {
		query = query.Where(`"enrollment_offering_adjustment".request_child_id = ?`, *requestChildID)
	}
	var count int
	if err := query.Scan(ctx, &count); err != nil {
		return 0, wrapDatabase("count enrollment offering adjustments for deletion", err)
	}
	return count, nil
}

func (r *enrollmentOfferingAdjustmentRepository) ListByRequestChildID(ctx context.Context, requestChildID int64) ([]*audit.EnrollmentOfferingAdjustment, error) {
	if requestChildID <= 0 {
		return nil, fmt.Errorf("request_child_id is required")
	}
	var rows []*audit.EnrollmentOfferingAdjustment
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&rows).
		ModelTableExpr(enrollmentOfferingAdjustmentTableExpr).
		Where(`"enrollment_offering_adjustment".request_child_id = ?`, requestChildID).
		OrderExpr(`"enrollment_offering_adjustment".changed_at DESC, "enrollment_offering_adjustment".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, wrapDatabase("list enrollment offering adjustments", err)
	}
	return rows, nil
}

// ListDirectForTenant returns the tenant's direct corrections newest first.
// Request-applied rows are excluded: the central history already shows those as
// the decided request they belong to (#2436).
func (r *enrollmentOfferingAdjustmentRepository) ListDirectForTenant(
	ctx context.Context,
	filters audit.DirectAdjustmentFilter,
) ([]*audit.EnrollmentOfferingAdjustment, error) {
	if filters.Limit <= 0 {
		return []*audit.EnrollmentOfferingAdjustment{}, nil
	}
	var rows []*audit.EnrollmentOfferingAdjustment
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&rows).
		ModelTableExpr(enrollmentOfferingAdjustmentTableExpr).
		Where(`"enrollment_offering_adjustment".source = ?`, audit.OfferingAdjustmentSourceDirect)
	if tenantID := runtimeTenantID(ctx, r.runtime); tenantID > 0 {
		query = query.Where(`"enrollment_offering_adjustment".tenant_id = ?`, tenantID)
	}
	if !filters.BeforeInstant.IsZero() {
		query = query.Where(`("enrollment_offering_adjustment".changed_at, "enrollment_offering_adjustment".id) < (?, ?)`, filters.BeforeInstant, filters.BeforeID)
	}
	query = query.
		OrderExpr(`"enrollment_offering_adjustment".changed_at DESC, "enrollment_offering_adjustment".id DESC`).
		Limit(filters.Limit)

	if err := query.Scan(ctx); err != nil {
		return nil, wrapDatabase("list direct enrollment offering adjustments", err)
	}
	return rows, nil
}
