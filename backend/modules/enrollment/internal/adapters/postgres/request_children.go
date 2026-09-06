package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

const requestChildTableExpr = `enrollment.request_children AS "request_child"`

func (r *Store) InsertChild(ctx context.Context, child *enrollment.RequestChild) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if child.TenantID != 0 && child.TenantID != tenantID {
		return fmt.Errorf("request child tenant mismatch")
	}
	if _, err := r.RequestByID(ctx, child.RequestID, false); err != nil {
		return fmt.Errorf("find child request: %w", err)
	}
	if child.RolloverSourceChildID != nil {
		if _, err := r.ChildByID(ctx, *child.RolloverSourceChildID); err != nil {
			return fmt.Errorf("find rollover source child: %w", err)
		}
	}
	row := childStorage(child)
	row.TenantID = tenantID

	_, err = db.NewInsert().
		Model(row).
		ModelTableExpr(requestChildTableExpr).
		Returning("*").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create request child: %w", err)
	}
	*child = *row.value()
	return nil
}
func (r *Store) ChildByID(ctx context.Context, id int64) (*enrollment.RequestChild, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	child := new(requestChildRow)
	err = db.NewSelect().
		Model(child).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Where(`"request_child".id = ?`, id).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find request child %d: %w", id, err)
	}
	return child.value(), nil
}
func (r *Store) ChildrenByID(ctx context.Context, ids []int64) ([]*enrollment.RequestChild, error) {
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
	var children []requestChildRow
	if err = db.NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Where(`"request_child".id IN (?)`, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list request children by ids: %w", err)
	}
	var result []*enrollment.RequestChild
	for _, row := range children {
		result = append(result, row.value())
	}
	return result, nil
}
func (r *Store) ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*enrollment.RequestChild, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var children []requestChildRow
	q := db.NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Where(`"request_child".request_id = ?`, requestID).
		OrderExpr(`"request_child".sort_order, "request_child".id`)
	if forUpdate {
		q = q.For("UPDATE")
	}
	err = q.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children: %w", err)
	}
	var result []*enrollment.RequestChild
	for _, row := range children {
		result = append(result, row.value())
	}
	return result, nil
}
func (r *Store) ChildrenForRequests(ctx context.Context, requestIDs []int64) ([]*enrollment.RequestChild, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if len(requestIDs) == 0 {
		return nil, nil
	}
	var children []requestChildRow
	err = db.NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Where(`"request_child".request_id IN (?)`, bun.List(requestIDs)).
		OrderExpr(`"request_child".request_id, "request_child".sort_order, "request_child".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children by request ids: %w", err)
	}
	var result []*enrollment.RequestChild
	for _, row := range children {
		result = append(result, row.value())
	}
	return result, nil
}
func (r *Store) ChildrenByPhaseStatuses(ctx context.Context, phaseID int64, statuses []string) ([]*enrollment.RequestChild, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	if phaseID <= 0 {
		return nil, fmt.Errorf("phase id must be positive")
	}
	if len(statuses) == 0 {
		return nil, nil
	}
	var children []requestChildRow
	err = db.NewSelect().
		Model(&children).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Join(`INNER JOIN enrollment.requests AS "request" ON "request".id = "request_child".request_id AND "request".tenant_id = "request_child".tenant_id`).
		Where(`"request".phase_id = ?`, phaseID).
		Where(`"request_child".status IN (?)`, bun.List(statuses)).
		OrderExpr(`"request_child".request_id, "request_child".sort_order, "request_child".id`).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request children by phase + status: %w", err)
	}
	var result []*enrollment.RequestChild
	for _, row := range children {
		result = append(result, row.value())
	}
	return result, nil
}
func (r *Store) UpdateChildData(ctx context.Context, child *enrollment.RequestChild) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if child == nil || child.ID <= 0 {
		return fmt.Errorf("request child id is required")
	}
	res, err := db.NewUpdate().
		Model(childStorage(child)).
		ModelTableExpr(requestChildTableExpr).
		Where(`"request_child".tenant_id = ?`, tenantID).
		Set("first_name = ?", child.FirstName).
		Set("last_name = ?", child.LastName).
		Set("date_of_birth = ?", child.DateOfBirth).
		Set("target_grade_level = ?", child.TargetGradeLevel).
		Set("target_school_class = ?", child.TargetSchoolClass).
		Set("custom_data = ?", child.CustomData).
		Set("sort_order = ?", child.SortOrder).
		Set("updated_at = NOW()").
		Where(`"request_child".id = ?`, child.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update request child data: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("request child %d not found", child.ID)
	}
	return nil
}
