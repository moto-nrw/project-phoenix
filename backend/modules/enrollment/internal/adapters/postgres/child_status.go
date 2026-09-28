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
		// Leaving the open statuses resolves whatever left the row to the
		// school, such as a renewal held for the Kinderkontingent (#3570).
		// While it stays open (a parent edit, "In Prüfung") the reason stays.
		Set("review_reason = CASE WHEN ? IN ('submitted', 'under_review') THEN review_reason ELSE NULL END", newStatus).
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

// HoldAutoRenewedChild moves one auto_renewed child to submitted and records
// why the automatic approval left it to the school. It changes nothing when
// the child is no longer auto_renewed.
func (r *Store) HoldAutoRenewedChild(ctx context.Context, id int64, reviewReason string) (bool, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}
	res, err := db.NewUpdate().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("status = ?", "submitted").
		Set("review_reason = ?", reviewReason).
		Set("updated_at = NOW()").
		Where(`"request_child".id = ?`, id).
		Where(`"request_child".status = ?`, "auto_renewed").
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to hold auto-renewed request child: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
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
