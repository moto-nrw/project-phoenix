package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// unreadRequestPredicate selects the requests that count as unread for one
// account (#3778): an active phase, at least one non-terminal child, and no
// read row at or after the last parent change. Arguments: tenant, account.
const unreadRequestPredicate = `"request".tenant_id = ?
	AND EXISTS (SELECT 1 FROM enrollment.phases AS "phase"
		WHERE "phase".id = "request".phase_id AND "phase".tenant_id = "request".tenant_id AND "phase".is_active)
	AND EXISTS (SELECT 1 FROM enrollment.request_children AS "child"
		WHERE "child".request_id = "request".id AND "child".tenant_id = "request".tenant_id
		AND "child".status NOT IN ('approved', 'rejected', 'withdrawn'))
	AND NOT EXISTS (SELECT 1 FROM enrollment.request_reads AS "read"
		WHERE "read".request_id = "request".id AND "read".tenant_id = "request".tenant_id
		AND "read".account_id = ?
		AND "read".read_at >= "request".parent_changed_at)`

// CountUnreadRequests counts the unread enrollments of one account.
func (r *Store) CountUnreadRequests(ctx context.Context, accountID int64) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr(requestTableExpr).
		Where(unreadRequestPredicate, tenantID, accountID).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count unread enrollment requests: %w", err)
	}
	return int(count), nil
}

// UnreadRequestIDs keeps the ids among requestIDs that are unread for the
// account.
func (r *Store) UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error) {
	if len(requestIDs) == 0 {
		return nil, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	err = db.NewSelect().TableExpr(requestTableExpr).ColumnExpr(`"request".id`).
		Where(unreadRequestPredicate, tenantID, accountID).
		Where(`"request".id IN (?)`, bun.List(requestIDs)).
		Scan(ctx, &ids)
	if err != nil {
		return nil, fmt.Errorf("failed to list unread enrollment requests: %w", err)
	}
	return ids, nil
}

// MarkRequestsRead stamps the account's read row on the given requests of
// the tenant. Ids of other tenants match no request and are skipped.
func (r *Store) MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error {
	if len(requestIDs) == 0 {
		return nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`INSERT INTO enrollment.request_reads (tenant_id, request_id, account_id, read_at)
		SELECT "request".tenant_id, "request".id, ?, NOW()
		FROM enrollment.requests AS "request"
		WHERE "request".tenant_id = ? AND "request".id IN (?)
		ON CONFLICT (request_id, account_id) DO UPDATE SET read_at = EXCLUDED.read_at`,
		accountID, tenantID, bun.List(requestIDs)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark enrollment requests read: %w", err)
	}
	return nil
}

// MarkAllRequestsRead marks every unread enrollment of the account read.
func (r *Store) MarkAllRequestsRead(ctx context.Context, accountID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`INSERT INTO enrollment.request_reads (tenant_id, request_id, account_id, read_at)
		SELECT "request".tenant_id, "request".id, ?, NOW()
		FROM enrollment.requests AS "request"
		WHERE `+unreadRequestPredicate+`
		ON CONFLICT (request_id, account_id) DO UPDATE SET read_at = EXCLUDED.read_at`,
		accountID, tenantID, accountID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark all enrollment requests read: %w", err)
	}
	return nil
}

// MarkRequestUnread drops the account's read row of one request.
func (r *Store) MarkRequestUnread(ctx context.Context, accountID, requestID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewDelete().TableExpr(`enrollment.request_reads AS "read"`).
		Where(`"read".tenant_id = ?`, tenantID).
		Where(`"read".account_id = ?`, accountID).
		Where(`"read".request_id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark enrollment request unread: %w", err)
	}
	return nil
}

// MarkRequestParentChanged makes the request unread for everyone again: a
// parent submitted, edited or confirmed it.
func (r *Store) MarkRequestParentChanged(ctx context.Context, requestID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewUpdate().TableExpr(requestTableExpr).
		Set("parent_changed_at = clock_timestamp()").
		Where(`"request".tenant_id = ?`, tenantID).
		Where(`"request".id = ?`, requestID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to record enrollment parent change: %w", err)
	}
	return nil
}
