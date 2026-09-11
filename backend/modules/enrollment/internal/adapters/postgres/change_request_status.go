package postgres

import (
	"context"
	"fmt"
	"time"
)

func (r *Store) SetChangeRequestStatus(ctx context.Context, id int64, status string) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	res, err := db.NewUpdate().TableExpr("enrollment.change_requests").
		Set("status = ?", status).Set("updated_at = NOW()").
		Where("id = ?", id).Where("tenant_id = ?", tenantID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update enrollment change request status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("enrollment change request %d not found", id)
	}
	return nil
}

func (r *Store) MarkChangeRequestReviewed(ctx context.Context, id int64, status string, note *string, reviewerID int64, at time.Time) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	q := db.NewUpdate().TableExpr("enrollment.change_requests").
		Set("status = ?", status).Set("admin_decision_note = ?", note).
		Set("reviewed_at = ?", at).Set("updated_at = NOW()").
		Where("id = ?", id).Where("tenant_id = ?", tenantID)
	if reviewerID > 0 {
		q = q.Set("reviewed_by_account_id = ?", reviewerID)
	} else {
		q = q.Set("reviewed_by_account_id = NULL")
	}
	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark enrollment change request reviewed: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("enrollment change request %d not found", id)
	}
	return nil
}
