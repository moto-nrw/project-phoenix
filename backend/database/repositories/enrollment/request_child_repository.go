package enrollment

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestChildTableExpr = `enrollment.request_children AS "request_child"`

type RequestChildRepository struct {
	db *bun.DB
}

func NewRequestChildRepository(db *bun.DB) enrollment.RequestChildRepository {
	return &RequestChildRepository{db: db}
}

func (r *RequestChildRepository) Create(ctx context.Context, child *enrollment.RequestChild) error {
	base.EnsureTenantID(ctx, child)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(child).
		ModelTableExpr(requestChildTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request child: %w", err)
	}
	return nil
}

// ListByRequestID returns all children for a request, sorted by sort_order.
func (r *RequestChildRepository) ListByRequestID(ctx context.Context, requestID int64) ([]*enrollment.RequestChild, error) {
	var children []*enrollment.RequestChild
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".request_id = ?`, requestID).
		OrderExpr(`"request_child".sort_order, "request_child".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children: %w", err)
	}
	return children, nil
}

// UpdateStatus transitions a single child to a new status. PR 8's
// decision service is the primary caller; PR 5 ships this so the schema
// and shape are pinned.
func (r *RequestChildRepository) UpdateStatus(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy int64) error {
	now := time.Now()
	q := base.GetDB(ctx, r.db).NewUpdate().
		Model((*enrollment.RequestChild)(nil)).
		ModelTableExpr(requestChildTableExpr).
		Set("status = ?", newStatus).
		Set("status_reason = ?", reason).
		Set("reviewed_at = ?", now).
		Set("reviewed_by = ?", reviewedBy).
		Where(`"request_child".id = ?`, id)

	res, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child status: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("request child %d not found", id)
	}
	return nil
}
