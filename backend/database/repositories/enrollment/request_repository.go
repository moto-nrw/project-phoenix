package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/uptrace/bun"
)

const requestTableExpr = `enrollment.requests AS "request"`

type RequestRepository struct {
	db *bun.DB
}

func NewRequestRepository(db *bun.DB) enrollment.RequestRepository {
	return &RequestRepository{db: db}
}

func (r *RequestRepository) Create(ctx context.Context, req *enrollment.Request) error {
	base.EnsureTenantID(ctx, req)

	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment request: %w", err)
	}
	return nil
}

func (r *RequestRepository) FindByID(ctx context.Context, id int64) (*enrollment.Request, error) {
	req := new(enrollment.Request)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Where(`"request".id = ?`, id).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("enrollment request %d not found", id)
		}
		return nil, fmt.Errorf("failed to find enrollment request: %w", err)
	}
	return req, nil
}

// ListAdmin returns every request for the tenant in context, newest
// first. Tenant-scoped via RLS. ChildStatus filter joins through the
// children table; an empty filter returns every row.
func (r *RequestRepository) ListAdmin(ctx context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	var rows []*enrollment.Request
	q := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(requestTableExpr)

	if filters.PhaseID > 0 {
		q = q.Where(`"request".phase_id = ?`, filters.PhaseID)
	}
	if filters.ChildStatus != "" {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.status = ?)`, filters.ChildStatus)
	}
	q = q.OrderExpr(`"request".submitted_at DESC, "request".id DESC`)

	if err := q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list admin requests: %w", err)
	}
	return rows, nil
}

// FindByStatusToken looks up a request by its status_token. Used by the
// public status/edit page (PR 7). Public route — caller must wrap in
// WithAdminTx because the token is the only auth signal.
func (r *RequestRepository) FindByStatusToken(ctx context.Context, token string) (*enrollment.Request, error) {
	req := new(enrollment.Request)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Where(`"request".status_token = ?`, token).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("enrollment request with token not found")
		}
		return nil, fmt.Errorf("failed to find enrollment request by token: %w", err)
	}
	return req, nil
}
