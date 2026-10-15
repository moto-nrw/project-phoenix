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
	return int(count), nil
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

// SettledRequestsBefore lists the requests no child was created from whose
// children are all settled past their windows, with at least one withdrawn
// (#3960). A withdrawal settles at its latest stamp: the parent's withdrawal
// stamps reviewed_at, the rollover deadline only updated_at. A rejection
// keeps the window of rejected enrollments; fully rejected requests stay
// with FullyRejectedRequestsBefore.
func (r *Store) SettledRequestsBefore(ctx context.Context, withdrawnCutoff, rejectedCutoff time.Time) ([]int64, error) {
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
		Having("BOOL_OR(rc.status = ?)", "withdrawn").
		Having("BOOL_AND(rc.created_student_id IS NULL)").
		Having(`BOOL_AND(CASE rc.status
			WHEN 'withdrawn' THEN GREATEST(rc.reviewed_at, rc.updated_at) < ?
			WHEN 'rejected' THEN rc.reviewed_at IS NOT NULL AND rc.reviewed_at < ?
			ELSE FALSE END)`, withdrawnCutoff, rejectedCutoff).
		OrderExpr("request.id").Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list settled enrollment requests: %w", err)
	}
	return ids, nil
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
