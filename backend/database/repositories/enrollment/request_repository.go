package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

// FindActiveDuplicate returns the subset of `children` that already
// have an active (non-rejected, non-withdrawn) enrollment in the given
// phase under the given guardian email. Comparison is
// LOWER(TRIM(...))-based on all three fields. The query joins
// requests → request_children and excludes child rows whose status is
// terminal-negative; an `approved` child still counts as a duplicate
// since the child is already enrolled. Tenant-scoped via RLS — caller
// must already be in a tenant-tx.
func (r *RequestRepository) FindActiveDuplicate(ctx context.Context, phaseID int64, guardianEmail string, children []enrollment.DuplicateChildKey) ([]enrollment.DuplicateChildKey, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase id must be positive")
	}
	email := strings.ToLower(strings.TrimSpace(guardianEmail))
	if email == "" {
		return nil, nil
	}
	if len(children) == 0 {
		return nil, nil
	}

	type row struct {
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	var rows []row

	q := base.GetDB(ctx, r.db).NewSelect().
		ColumnExpr(`LOWER(TRIM(rc.first_name)) AS first_name`).
		ColumnExpr(`LOWER(TRIM(rc.last_name)) AS last_name`).
		TableExpr(`enrollment.request_children AS rc`).
		Join(`JOIN enrollment.requests AS req ON req.id = rc.request_id`).
		Where(`req.phase_id = ?`, phaseID).
		Where(`LOWER(TRIM(req.guardian_email)) = ?`, email).
		Where(`rc.status NOT IN (?, ?)`, enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn)

	conds := make([]string, 0, len(children))
	args := make([]any, 0, len(children)*2)
	for _, c := range children {
		fn := strings.ToLower(strings.TrimSpace(c.FirstName))
		ln := strings.ToLower(strings.TrimSpace(c.LastName))
		if fn == "" || ln == "" {
			continue
		}
		conds = append(conds, `(LOWER(TRIM(rc.first_name)) = ? AND LOWER(TRIM(rc.last_name)) = ?)`)
		args = append(args, fn, ln)
	}
	if len(conds) == 0 {
		return nil, nil
	}
	q = q.Where("("+strings.Join(conds, " OR ")+")", args...)

	if err := q.Distinct().Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to check duplicate enrollments: %w", err)
	}

	out := make([]enrollment.DuplicateChildKey, 0, len(rows))
	for _, rr := range rows {
		out = append(out, enrollment.DuplicateChildKey{FirstName: rr.FirstName, LastName: rr.LastName})
	}
	return out, nil
}

// ExistsByPhaseID returns true if any request row references the given
// phase. Powers the phase-delete safety check — admin sees a 409 instead
// of cascading every parent submission into oblivion.
func (r *RequestRepository) ExistsByPhaseID(ctx context.Context, phaseID int64) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".phase_id = ?`, phaseID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check phase references in requests: %w", err)
	}
	return count > 0, nil
}

// CountByPhaseID returns how many request rows reference the given
// phase. Powers the phase-delete confirmation modal ("X Anmeldungen
// werden gelöscht"). Tenant-scoped via RLS.
func (r *RequestRepository) CountByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".phase_id = ?`, phaseID).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count phase requests: %w", err)
	}
	return count, nil
}

// DeleteByPhaseID removes every request row for the phase. The FK
// cascade drops request_children and request_child_offerings with it;
// the request_child_offerings.care_offering_id ON DELETE RESTRICT means
// callers MUST delete requests before deleting the phase's care
// offerings (or the phase itself, which cascades the offerings).
// users.students rows are untouched — request_children.created_student_id
// is ON DELETE SET NULL, and the student is the parent in that
// relationship, so deleting children never deletes the student.
// Tenant-scoped via RLS.
func (r *RequestRepository) DeleteByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	res, err := base.GetDB(ctx, r.db).NewDelete().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".phase_id = ?`, phaseID).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to delete phase requests: %w", err)
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

// ExistsBySchemaID returns true if any request row references the
// schema version. Used to keep historical enrollment submissions
// auditable when admins delete unused form templates.
func (r *RequestRepository) ExistsBySchemaID(ctx context.Context, schemaID int64) (bool, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*enrollment.Request)(nil)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".schema_id = ?`, schemaID).
		Count(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check schema references in requests: %w", err)
	}
	return count > 0, nil
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
