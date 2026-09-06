package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) ActiveDuplicateChildren(ctx context.Context, phaseID int64, guardianEmail string, children []enrollment.DuplicateChildKey, excludedRequestID int64) ([]enrollment.DuplicateChildKey, error) {
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

	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	q := db.NewSelect().
		ColumnExpr(`LOWER(TRIM(rc.first_name)) AS first_name`).
		ColumnExpr(`LOWER(TRIM(rc.last_name)) AS last_name`).
		TableExpr(`enrollment.request_children AS rc`).
		Join(`JOIN enrollment.requests AS req ON req.id = rc.request_id AND req.tenant_id = rc.tenant_id`).
		Where(`req.tenant_id = ?`, tenantID).
		Where(`req.phase_id = ?`, phaseID).
		Where(`LOWER(TRIM(req.guardian_email)) = ?`, email).
		Where(`rc.status NOT IN (?, ?)`, "rejected", "withdrawn")
	if excludedRequestID > 0 {
		q = q.Where(`req.id <> ?`, excludedRequestID)
	}

	keys := make([]enrollment.DuplicateChildKey, 0, len(children))
	for _, c := range children {
		fn := strings.ToLower(strings.TrimSpace(c.FirstName))
		ln := strings.ToLower(strings.TrimSpace(c.LastName))
		if fn == "" || ln == "" {
			continue
		}
		keys = append(keys, enrollment.DuplicateChildKey{FirstName: fn, LastName: ln})
	}
	if len(keys) == 0 {
		return nil, nil
	}
	q = q.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
		for _, key := range keys {
			group = group.WhereOr(`(LOWER(TRIM(rc.first_name)) = ? AND LOWER(TRIM(rc.last_name)) = ?)`, key.FirstName, key.LastName)
		}
		return group
	})

	if err := q.Distinct().Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("failed to check duplicate enrollments: %w", err)
	}

	out := make([]enrollment.DuplicateChildKey, 0, len(rows))
	for _, rr := range rows {
		out = append(out, enrollment.DuplicateChildKey{FirstName: rr.FirstName, LastName: rr.LastName})
	}
	return out, nil
}

func (r *Store) HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludeRequestChildID int64) (bool, error) {
	if phaseID <= 0 || studentID <= 0 {
		return false, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return false, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return false, err
	}
	q := db.NewSelect().
		TableExpr(`enrollment.request_children AS rc`).
		Join(`JOIN enrollment.requests AS req ON req.id = rc.request_id AND req.tenant_id = rc.tenant_id`).
		Where(`req.tenant_id = ?`, tenantID).
		Where(`req.phase_id = ?`, phaseID).
		Where(`rc.matched_student_id = ?`, studentID).
		Where(`rc.status NOT IN (?, ?)`, "rejected", "withdrawn")
	if excludeRequestChildID > 0 {
		q = q.Where(`rc.id <> ?`, excludeRequestChildID)
	}
	exists, err := q.Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check active request for matched student: %w", err)
	}
	return exists, nil
}
