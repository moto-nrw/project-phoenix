package postgres

import (
	"context"
	"fmt"
)

func (r *Store) DeleteRequestChildren(ctx context.Context, requestID int64) error {
	if requestID <= 0 {
		return fmt.Errorf("request id must be positive")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().TableExpr("enrollment.request_children").Where("tenant_id = ? AND request_id = ?", tenantID, requestID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete request children: %w", err)
	}
	return nil
}

func (r *Store) CountCreatedStudentsByPhase(ctx context.Context, phaseID int64) (int, error) {
	if phaseID <= 0 {
		return 0, fmt.Errorf("phase id must be positive")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = db.NewSelect().TableExpr("enrollment.request_children AS child").
		Join("JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id").
		Where("child.tenant_id = ? AND request.phase_id = ?", tenantID, phaseID).
		Where("child.created_student_id IS NOT NULL").ColumnExpr("COUNT(DISTINCT child.created_student_id)").Scan(ctx, &count)
	if err != nil {
		return 0, fmt.Errorf("failed to count created students by phase: %w", err)
	}
	return count, nil
}
