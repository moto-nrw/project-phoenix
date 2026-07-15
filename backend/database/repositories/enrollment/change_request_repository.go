package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const changeRequestTableExpr = `enrollment.change_requests AS "change_request"`

type ChangeRequestRepository struct {
	db *bun.DB
}

func NewChangeRequestRepository(db *bun.DB) enrollment.ChangeRequestRepository {
	return &ChangeRequestRepository{db: db}
}

func (r *ChangeRequestRepository) Create(ctx context.Context, row *enrollment.ChangeRequest) error {
	base.EnsureTenantID(ctx, row)
	if row.Status == "" {
		row.Status = enrollment.ChangeRequestStatusPendingReview
	}
	if row.Origin == "" {
		row.Origin = enrollment.ChangeRequestOriginParent
	}
	if row.BaseSnapshot == nil {
		row.BaseSnapshot = map[string]any{}
	}
	if row.ProposedSnapshot == nil {
		row.ProposedSnapshot = map[string]any{}
	}
	if row.Diff == nil {
		row.Diff = map[string]any{}
	}
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(row).
		ModelTableExpr(changeRequestTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment change request: %w", err)
	}
	return nil
}

func (r *ChangeRequestRepository) FindByID(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	return r.findByID(ctx, id, "")
}

func (r *ChangeRequestRepository) FindByIDForUpdate(ctx context.Context, id int64) (*enrollment.ChangeRequest, error) {
	return r.findByID(ctx, id, "UPDATE")
}

func (r *ChangeRequestRepository) findByID(ctx context.Context, id int64, lockClause string) (*enrollment.ChangeRequest, error) {
	row := new(enrollment.ChangeRequest)
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(row).
		ModelTableExpr(changeRequestTableExpr).
		Where(`"change_request".id = ?`, id)
	if lockClause != "" {
		q = q.For(lockClause)
	}
	err := q.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("enrollment change request %d not found", id)
		}
		return nil, fmt.Errorf("failed to find enrollment change request: %w", err)
	}
	return row, nil
}

func (r *ChangeRequestRepository) ListByRequestID(ctx context.Context, requestID int64) ([]*enrollment.ChangeRequest, error) {
	var rows []*enrollment.ChangeRequest
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(changeRequestTableExpr).
		Where(`"change_request".request_id = ?`, requestID).
		OrderExpr(`"change_request".created_at DESC, "change_request".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list enrollment change requests by request: %w", err)
	}
	return rows, nil
}

func (r *ChangeRequestRepository) ListOpenByRequestIDForUpdate(ctx context.Context, requestID int64) ([]*enrollment.ChangeRequest, error) {
	var rows []*enrollment.ChangeRequest
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(changeRequestTableExpr).
		Where(`"change_request".request_id = ?`, requestID).
		Where(`"change_request".status IN (?, ?)`,
			enrollment.ChangeRequestStatusPendingReview,
			enrollment.ChangeRequestStatusNeedsParentResponse,
		).
		OrderExpr(`"change_request".created_at ASC, "change_request".id ASC`).
		For("UPDATE").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to lock open enrollment change requests by request: %w", err)
	}
	return rows, nil
}

func (r *ChangeRequestRepository) ListAdmin(ctx context.Context, filters enrollment.ChangeRequestListFilters) ([]*enrollment.ChangeRequest, error) {
	var rows []*enrollment.ChangeRequest
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(changeRequestTableExpr)
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
	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list enrollment change requests: %w", err)
	}
	return rows, nil
}

func (r *ChangeRequestRepository) MarkReviewed(ctx context.Context, id int64, status string, note *string, reviewerAccountID int64, reviewedAt time.Time) error {
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.ChangeRequest)(nil)).
		ModelTableExpr(changeRequestTableExpr).
		Set("status = ?", status).
		Set("admin_decision_note = ?", note).
		Set("reviewed_at = ?", reviewedAt).
		Set("updated_at = NOW()").
		Where(`"change_request".id = ?`, id)
	if reviewerAccountID > 0 {
		q = q.Set("reviewed_by_account_id = ?", reviewerAccountID)
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

func (r *ChangeRequestRepository) SetStatus(ctx context.Context, id int64, status string) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.ChangeRequest)(nil)).
		ModelTableExpr(changeRequestTableExpr).
		Set("status = ?", status).
		Set("updated_at = NOW()").
		Where(`"change_request".id = ?`, id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update enrollment change request status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("enrollment change request %d not found", id)
	}
	return nil
}
