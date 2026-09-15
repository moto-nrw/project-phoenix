package postgres

import (
	"context"
	"fmt"
	"time"
)

func (r *Store) LinkCreatedStudent(ctx context.Context, requestChildID, studentID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("created_student_id = ?", studentID).
		Where(`"request_child".id = ?`, requestChildID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to link created student: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request child %d not found", requestChildID)
	}
	return nil
}
func (r *Store) UpdateMatchedStudent(ctx context.Context, requestChildID int64, studentID *int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("matched_student_id = ?", studentID).
		Set("updated_at = ?", time.Now()).
		Where(`"request_child".id = ?`, requestChildID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child matched student: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request child %d not found", requestChildID)
	}
	return nil
}
