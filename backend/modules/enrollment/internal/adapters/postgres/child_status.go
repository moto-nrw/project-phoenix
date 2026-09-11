package postgres

import (
	"context"
	"fmt"
	"time"
)

func (r *Store) UpdateChildStatus(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	q := db.NewUpdate().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("status = ?", newStatus).
		Set("status_reason = ?", reason).
		Set("reviewed_at = ?", now).
		Where(`"request_child".id = ?`, id)
	if reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", reviewedBy)
	} else {
		// Parent-initiated transitions (e.g., self-withdraw) carry no
		// reviewer. Setting NULL avoids a FK violation against
		// auth.accounts(id).
		q = q.Set("reviewed_by = NULL")
	}

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}
func (r *Store) ReviewRolloverChild(ctx context.Context, id int64, newStatus string, reason *string, newGradeLevel *int16, reviewedBy int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	q := db.NewUpdate().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("status = ?", newStatus).
		Set("status_reason = ?", reason).
		Set("reviewed_at = ?", now).
		Where(`"request_child".id = ?`, id)
	if newGradeLevel != nil {
		q = q.Set("target_grade_level = ?", *newGradeLevel)
	}
	if reviewedBy > 0 {
		q = q.Set("reviewed_by = ?", reviewedBy)
	}
	// Clear review_reason once the row leaves admin review.
	q = q.Set("review_reason = NULL")

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update rollover review: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}
