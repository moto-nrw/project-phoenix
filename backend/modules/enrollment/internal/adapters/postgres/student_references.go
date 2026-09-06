package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]enrollment.CareExitApplicationLink, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	rows := []enrollment.CareExitApplicationLink{}
	query := db.NewSelect().TableExpr("enrollment.request_children").Column("id", "tenant_id", "created_student_id", "matched_student_id", "status").Where("tenant_id = ?", tenantID)
	if len(studentIDs) > 0 {
		query = query.Where("(created_student_id IN (?) OR matched_student_id IN (?))", bun.List(studentIDs), bun.List(studentIDs))
	}
	if err := query.OrderExpr("id ASC").Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list care-exit application links: %w", err)
	}
	return rows, nil
}

func (r *Store) CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	if len(studentIDs) == 0 {
		return []int64{}, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	err = db.NewSelect().TableExpr("enrollment.request_children").Column("id").Where("tenant_id = ?", tenantID).Where("created_student_id IN (?)", bun.List(studentIDs)).OrderExpr("id ASC").Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("list enrollment children created as students: %w", err)
	}
	return ids, nil
}

func (r *Store) CountStudentReferences(ctx context.Context, studentID int64) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("enrollment.request_children").Where("tenant_id = ?", tenantID).Where("(created_student_id = ? OR matched_student_id = ?)", studentID, studentID).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count enrollment student references: %w", err)
	}
	return count, nil
}
