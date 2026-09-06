package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func (r *Store) RestoreWithdrawnChildren(ctx context.Context, requestID int64, waitlistedChildIDs []int64) ([]int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if requestID <= 0 {
		return nil, fmt.Errorf("request id is required")
	}
	restored := make([]int64, 0)
	if len(waitlistedChildIDs) > 0 {
		var waitlisted []int64
		err := db.NewUpdate().
			TableExpr(`enrollment.request_children AS "request_child"`).
			Where(`"request_child".tenant_id = ?`, tenantID).
			Set("status = ?", "waitlisted").
			Set("status_reason = NULL").
			Set("reviewed_at = NULL").
			Set("reviewed_by = NULL").
			Set("updated_at = NOW()").
			Where(`"request_child".request_id = ?`, requestID).
			Where(`"request_child".status = ?`, "withdrawn").
			Where(`"request_child".id IN (?)`, bun.List(waitlistedChildIDs)).
			Returning(`"request_child".id`).
			Scan(ctx, &waitlisted)
		if err != nil {
			return nil, fmt.Errorf("failed to restore withdrawn request children to waitlist: %w", err)
		}
		restored = append(restored, waitlisted...)
	}
	var submitted []int64
	q := db.NewUpdate().
		TableExpr(`enrollment.request_children AS "request_child"`).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("status = ?", "submitted").
		Set("status_reason = NULL").
		Set("reviewed_at = NULL").
		Set("reviewed_by = NULL").
		Set("updated_at = NOW()").
		Where(`"request_child".request_id = ?`, requestID).
		Where(`"request_child".status = ?`, "withdrawn")
	if len(waitlistedChildIDs) > 0 {
		q = q.Where(`"request_child".id NOT IN (?)`, bun.List(waitlistedChildIDs))
	}
	if err := q.Returning(`"request_child".id`).Scan(ctx, &submitted); err != nil {
		return nil, fmt.Errorf("failed to restore withdrawn request children: %w", err)
	}
	return append(restored, submitted...), nil
}
func (r *Store) TransitionPhaseChildren(ctx context.Context, phaseID int64, currentStatus, newStatus string) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	if phaseID <= 0 {
		return 0, fmt.Errorf("phase id must be positive")
	}
	if currentStatus == "" || newStatus == "" {
		return 0, fmt.Errorf("current and new status are required")
	}

	res, err := db.NewRaw(`
		UPDATE enrollment.request_children AS rc
		SET status = ?, updated_at = NOW()
		FROM enrollment.requests AS r
		WHERE r.id = rc.request_id AND r.tenant_id = rc.tenant_id AND rc.tenant_id = ?
		  AND r.phase_id = ?
		  AND rc.status = ?
	`, newStatus, tenantID, phaseID, currentStatus).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("bulk update status: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}
