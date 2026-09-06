package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

const changeRequestTableExpr = `enrollment.change_requests AS "change_request"`
const changeRequestColumns = `"change_request".id, "change_request".tenant_id, "change_request".created_at, "change_request".updated_at, request_id, request_child_id, origin, status, parent_note, admin_decision_note, base_snapshot, proposed_snapshot, diff_json AS diff, care_offerings_enabled_at_creation, created_by_account_id, reviewed_by_account_id, reviewed_at`

func (r *Store) ChangeRequestByID(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	return r.findChangeRequestByID(ctx, id, "")
}

func (r *Store) ChangeRequestByIDForUpdate(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	return r.findChangeRequestByID(ctx, id, "UPDATE")
}

func (r *Store) findChangeRequestByID(ctx context.Context, id int64, lockClause string) (*enrollment.ChangeRequest, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	row := new(enrollment.ChangeRequest)
	q := db.NewSelect().
		TableExpr(changeRequestTableExpr).
		ColumnExpr(changeRequestColumns).
		Where(`"change_request".tenant_id = ?`, tenantID).
		Where(`"change_request".id = ?`, id)
	if lockClause != "" {
		q = q.For(lockClause)
	}
	err = q.Scan(ctx, row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("enrollment change request %d not found", id)
		}
		return nil, fmt.Errorf("failed to find enrollment change request: %w", err)
	}
	return row, nil
}

func (r *Store) ChangeRequestsForRequest(ctx context.Context, requestID int64) ([]*enrollment.ChangeRequest, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*enrollment.ChangeRequest
	err = db.NewSelect().
		TableExpr(changeRequestTableExpr).
		ColumnExpr(changeRequestColumns).
		Where(`"change_request".tenant_id = ?`, tenantID).
		Where(`"change_request".request_id = ?`, requestID).
		OrderExpr(`"change_request".created_at DESC, "change_request".id DESC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to list enrollment change requests by request: %w", err)
	}
	return rows, nil
}

func (r *Store) OpenChangeRequestsForRequestForUpdate(ctx context.Context, requestID int64) ([]*enrollment.ChangeRequest, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*enrollment.ChangeRequest
	err = db.NewSelect().
		TableExpr(changeRequestTableExpr).
		ColumnExpr(changeRequestColumns).
		Where(`"change_request".tenant_id = ?`, tenantID).
		Where(`"change_request".request_id = ?`, requestID).
		Where(`"change_request".status IN (?, ?)`,
			"pending_review",
			"needs_parent_response",
		).
		OrderExpr(`"change_request".created_at ASC, "change_request".id ASC`).
		For("UPDATE").
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to lock open enrollment change requests by request: %w", err)
	}
	return rows, nil
}

func (r *Store) ListChangeRequests(ctx context.Context, filters enrollment.ChangeRequestListFilters) ([]*enrollment.ChangeRequest, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*enrollment.ChangeRequest
	q := db.NewSelect().
		TableExpr(changeRequestTableExpr).
		ColumnExpr(changeRequestColumns).
		Where(`"change_request".tenant_id = ?`, tenantID)
	if filters.RequestID > 0 {
		q = q.Where(`"change_request".request_id = ?`, filters.RequestID)
	}
	if filters.Status != "" {
		q = q.Where(`"change_request".status = ?`, filters.Status)
	}
	q = q.OrderExpr(`"change_request".created_at DESC, "change_request".id DESC`)
	if filters.Limit > 0 {
		q = q.Limit(filters.Limit)
	}
	if err := q.Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to list enrollment change requests: %w", err)
	}
	return rows, nil
}

// ChangeRequestsForReview serves the request module's Eltern tab (#2435): one page of
// change requests, newest first, keyset-paginated. The open list orders by
// created_at (when the family filed it), the history by the decision instant
// (when the office decided) — the same split the other four request queues use,
// so the merged list stays in one order across all types.
//
// The decided-at range filters COALESCE(reviewed_at, updated_at): a decision
// always stamps reviewed_at, and the fallback keeps rows that only ever got a
// status change inside the window they actually moved in.
func (r *Store) ChangeRequestsForReview(ctx context.Context, filters enrollment.ChangeRequestReviewFilters) ([]*enrollment.ChangeRequest, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if len(filters.Statuses) == 0 || filters.Limit <= 0 {
		return []*enrollment.ChangeRequest{}, nil
	}
	rows := make([]*enrollment.ChangeRequest, 0, filters.Limit)
	q := db.NewSelect().
		TableExpr(changeRequestTableExpr).
		ColumnExpr(changeRequestColumns).
		Where(`"change_request".tenant_id = ?`, tenantID).
		Where(`"change_request".status IN (?)`, bun.List(filters.Statuses))
	if filters.Search != "" {
		// The affected child is the pinned one when the request targets a
		// single child, and every child of the enrollment otherwise — the same
		// rule the response's child names follow, so a name that shows up in
		// the list is also a name that finds it.
		q = q.Where(`EXISTS (
			SELECT 1 FROM enrollment.request_children AS rc
			WHERE rc.request_id = "change_request".request_id
			  AND rc.tenant_id = "change_request".tenant_id
			  AND ("change_request".request_child_id IS NULL OR rc.id = "change_request".request_child_id)
			  AND (rc.first_name || ' ' || rc.last_name) ILIKE ? ESCAPE '\'
		)`, "%"+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(filters.Search)+"%")
	}
	if filters.History {
		if !filters.From.IsZero() {
			q = q.Where(`COALESCE("change_request".reviewed_at, "change_request".updated_at) >= ?`, filters.From)
		}
		if !filters.To.IsZero() {
			q = q.Where(`COALESCE("change_request".reviewed_at, "change_request".updated_at) <= ?`, filters.To)
		}
	}
	if !filters.BeforeInstant.IsZero() {
		if filters.History {
			q = q.Where(`(COALESCE("change_request".reviewed_at, "change_request".updated_at), "change_request".id) < (?, ?)`, filters.BeforeInstant, filters.BeforeID)
		} else {
			q = q.Where(`("change_request".created_at, "change_request".id) < (?, ?)`, filters.BeforeInstant, filters.BeforeID)
		}
	}
	if filters.History {
		q = q.OrderExpr(`COALESCE("change_request".reviewed_at, "change_request".updated_at) DESC`)
	} else {
		q = q.OrderExpr(`"change_request".created_at DESC`)
	}
	err = q.
		OrderExpr(`"change_request".id DESC`).
		Limit(filters.Limit).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to list enrollment change requests for review: %w", err)
	}
	return rows, nil
}
