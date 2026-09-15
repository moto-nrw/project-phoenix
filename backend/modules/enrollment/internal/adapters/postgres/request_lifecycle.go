package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func (r *Store) SetRequestWithdrawal(ctx context.Context, id int64, at *time.Time) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().TableExpr("enrollment.requests").
		Set("withdrawn_at = ?", at).Set("updated_at = NOW()").
		Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	if err != nil {
		if at == nil {
			return fmt.Errorf("failed to clear request withdrawn state: %w", err)
		}
		return fmt.Errorf("failed to mark request withdrawn: %w", err)
	}
	return nil
}

func (r *Store) CountPhaseRequests(ctx context.Context, id int64) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("enrollment.requests").Where("tenant_id = ? AND phase_id = ?", tenantID, id).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count phase requests: %w", err)
	}
	return count, nil
}
func (r *Store) DeletePhaseRequests(ctx context.Context, id int64) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	result, err := db.NewDelete().TableExpr("enrollment.requests").Where("tenant_id = ? AND phase_id = ?", tenantID, id).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete phase requests: %w", err)
	}
	count, err := result.RowsAffected()
	return int(count), err
}
func (r *Store) DeleteRequest(ctx context.Context, id int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	result, err := db.NewDelete().TableExpr("enrollment.requests").Where("tenant_id = ? AND id = ?", tenantID, id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete enrollment request: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("enrollment request not found for cleanup")
	}
	return nil
}
func (r *Store) FullyRejectedRequestsBefore(ctx context.Context, cutoff time.Time) ([]int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	err = db.NewSelect().TableExpr("enrollment.requests AS request").ColumnExpr("request.id").
		Join("JOIN enrollment.request_children AS rc ON rc.request_id = request.id AND rc.tenant_id = request.tenant_id").
		Where("request.tenant_id = ?", tenantID).GroupExpr("request.id").
		Having("COUNT(rc.id) > 0").Having("BOOL_AND(rc.status = ?)", "rejected").
		Having("BOOL_AND(rc.reviewed_at IS NOT NULL AND rc.reviewed_at < ?)", cutoff).
		OrderExpr("request.id").Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list fully rejected enrollment requests: %w", err)
	}
	return ids, nil
}
