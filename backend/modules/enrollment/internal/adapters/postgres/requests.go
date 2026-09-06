package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

const requestTableExpr = `enrollment.requests AS "request"`

func (r *Store) InsertRequest(ctx context.Context, req *enrollment.Request) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}

	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if req.TenantID != 0 && req.TenantID != tenantID {
		return fmt.Errorf("request tenant mismatch")
	}
	if _, err := r.Phase(ctx, req.PhaseID); err != nil {
		return fmt.Errorf("find request phase: %w", err)
	}
	if req.SchemaID != nil {
		if _, err := r.Schema(ctx, *req.SchemaID); err != nil {
			return fmt.Errorf("find request schema: %w", err)
		}
	}
	row := requestStorage(req)
	row.TenantID = tenantID

	_, err = db.NewInsert().
		Model(row).
		ModelTableExpr(requestTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create enrollment request: %w", err)
	}
	*req = *row.value()
	return nil
}

func (r *Store) RequestsByID(ctx context.Context, ids []int64) ([]*enrollment.Request, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}

	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []requestRow
	if err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(requestTableExpr).
		Where(`"request".tenant_id = ?`, tenantID).
		Where(`"request".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list enrollment requests by ids: %w", err)
	}
	var result []*enrollment.Request
	for _, row := range rows {
		result = append(result, row.value())
	}
	return result, nil
}

func (r *Store) RequestByID(ctx context.Context, id int64, forUpdate bool) (*enrollment.Request, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}

	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	req := new(requestRow)
	query := db.NewSelect().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Where(`"request".tenant_id = ?`, tenantID).
		Where(`"request".id = ?`, id)
	if forUpdate {
		query = query.For("UPDATE")
	}
	err = query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Keep sql.ErrNoRows in the chain so services can tell a
			// missing row from a query failure (same contract as the
			// form schema repository).
			return nil, fmt.Errorf("enrollment request %d not found: %w", id, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("failed to find enrollment request: %w", err)
	}
	return req.value(), nil
}

func (r *Store) AdminRequests(ctx context.Context, filters enrollment.RequestListFilters) ([]*enrollment.Request, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}

	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestRow
	q := db.NewSelect().
		Model(&rows).
		ModelTableExpr(requestTableExpr).
		Where(`"request".tenant_id = ?`, tenantID)

	if filters.PhaseID > 0 {
		q = q.Where(`"request".phase_id = ?`, filters.PhaseID)
	}
	if filters.ChildStatus != "" {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.tenant_id = "request".tenant_id AND rc.status = ?)`, filters.ChildStatus)
	}
	if filters.CreatedStudentID > 0 {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.tenant_id = "request".tenant_id AND rc.created_student_id = ?)`, filters.CreatedStudentID)
	}
	if len(filters.CreatedStudentIDs) > 0 {
		q = q.Where(`EXISTS (SELECT 1 FROM enrollment.request_children rc WHERE rc.request_id = "request".id AND rc.tenant_id = "request".tenant_id AND rc.created_student_id IN (?))`, bun.List(filters.CreatedStudentIDs))
	}
	q = q.OrderExpr(`"request".submitted_at DESC, "request".id DESC`)

	if err = q.Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list admin requests: %w", err)
	}
	var result []*enrollment.Request
	for _, row := range rows {
		result = append(result, row.value())
	}
	return result, nil
}

func (r *Store) RequestByToken(ctx context.Context, token string, forUpdate bool) (*enrollment.Request, error) {
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	req := new(requestRow)
	q := db.NewSelect().
		Model(req).
		ModelTableExpr(requestTableExpr).
		Where(`"request".status_token = ?`, token)
	if forUpdate {
		q = q.For("UPDATE")
	}
	err = q.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("enrollment request with token not found")
		}
		return nil, fmt.Errorf("failed to find enrollment request by token: %w", err)
	}
	return req.value(), nil
}

func (r *Store) UpdateRequestGuardian(ctx context.Context, req *enrollment.Request, includeEmail bool) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}

	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	q := db.NewUpdate().
		Model(requestStorage(req)).
		ModelTableExpr(requestTableExpr).
		Where(`"request".tenant_id = ?`, tenantID).
		Set("guardian_first_name = ?", req.GuardianFirstName).
		Set("guardian_last_name = ?", req.GuardianLastName).
		Set("guardian_phone = ?", req.GuardianPhone).
		Set("consent_flags = ?", req.ConsentFlags).
		Set("legal_blocks_snapshot = ?", req.LegalBlocksSnapshot).
		Set("custom_data = ?", req.CustomData).
		Set("updated_at = NOW()").
		Where(`"request".id = ?`, req.ID)
	if includeEmail {
		q = q.Set("guardian_email = ?", req.GuardianEmail)
	}
	result, err := q.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update guardian data: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to update guardian data: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("failed to update guardian data: %w", sql.ErrNoRows)
	}
	return nil
}
