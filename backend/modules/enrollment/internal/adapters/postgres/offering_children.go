package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) OfferingChildren(ctx context.Context, childIDs []int64) ([]enrollment.OfferingChildFacts, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []enrollment.OfferingChildFacts
	err = db.NewSelect().TableExpr("enrollment.request_children").
		Column("id", "status", "target_grade_level", "created_student_id", "matched_student_id").
		Where("tenant_id = ? AND id IN (?)", tenantID, bun.List(childIDs)).OrderExpr("id").Scan(ctx, &rows)
	return rows, err
}

func (r *Store) ApprovedOfferingChildrenForStudents(ctx context.Context, studentIDs []int64) ([]enrollment.OfferingChildFacts, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []enrollment.OfferingChildFacts
	err = db.NewSelect().TableExpr("enrollment.request_children").
		Column("id", "status", "target_grade_level", "created_student_id", "matched_student_id").
		Where("tenant_id = ? AND status = ?", tenantID, enrollment.ChildStatusApproved).
		Where("COALESCE(created_student_id, matched_student_id) IN (?)", bun.List(studentIDs)).OrderExpr("id").Scan(ctx, &rows)
	return rows, err
}
