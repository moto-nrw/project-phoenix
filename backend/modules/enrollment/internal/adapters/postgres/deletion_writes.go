package postgres

import (
	"context"
	"fmt"
)

func (r *Store) DeleteRequestChildTree(ctx context.Context, requestID, childID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	// audit.enrollment_offering_adjustments rows are NOT deleted here:
	// phoenix_tenant only holds SELECT/INSERT on that append-only table.
	// Its request_child_id FK is ON DELETE CASCADE, so deleting the
	// request_children row below removes them.
	if _, err := db.NewRaw(`DELETE FROM enrollment.change_request_messages WHERE tenant_id = ? AND change_request_id IN (SELECT id FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ? AND request_child_id = ?)`, tenantID, tenantID, requestID, childID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment change request messages: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ? AND request_child_id = ?`, tenantID, requestID, childID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment change requests: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE tenant_id = ? AND request_child_id IN (SELECT id FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ? AND id = ?)`, tenantID, tenantID, requestID, childID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment child offerings: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ? AND id = ?`, tenantID, requestID, childID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment request child: %w", err)
	}
	return nil
}

func (r *Store) DeleteRequestTree(ctx context.Context, requestID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	// audit.enrollment_offering_adjustments rows are NOT deleted here:
	// phoenix_tenant only holds SELECT/INSERT on that append-only table.
	// Its request_id / request_child_id FKs are ON DELETE CASCADE, so
	// deleting the request_children and requests rows below removes them.
	if _, err := db.NewRaw(`DELETE FROM enrollment.change_request_messages WHERE tenant_id = ? AND change_request_id IN (SELECT id FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ?)`, tenantID, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment change request messages: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.change_requests WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment change requests: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.late_invites WHERE tenant_id = ? AND used_request_id = ?`, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment late invites: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.request_child_offerings WHERE tenant_id = ? AND request_child_id IN (SELECT id FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ?)`, tenantID, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment child offerings: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.request_guardians WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment request guardians: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.request_children WHERE tenant_id = ? AND request_id = ?`, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment request children: %w", err)
	}
	if _, err := db.NewRaw(`DELETE FROM enrollment.requests WHERE tenant_id = ? AND id = ?`, tenantID, requestID).Exec(ctx); err != nil {
		return fmt.Errorf("delete enrollment request: %w", err)
	}
	return nil
}
