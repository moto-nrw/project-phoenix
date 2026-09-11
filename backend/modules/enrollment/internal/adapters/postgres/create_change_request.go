package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (r *Store) InsertChangeRequest(ctx context.Context, row *enrollment.ChangeRequest) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	encoded := make([]string, 0, 3)
	for _, value := range []json.RawMessage{row.BaseSnapshot, row.ProposedSnapshot, row.Diff} {
		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to create enrollment change request: %w", err)
		}
		encoded = append(encoded, string(data))
	}
	err = db.NewRaw(`INSERT INTO enrollment.change_requests
		(tenant_id, request_id, request_child_id, origin, status, parent_note, admin_decision_note,
		base_snapshot, proposed_snapshot, diff_json, care_offerings_enabled_at_creation,
		created_by_account_id, reviewed_by_account_id, reviewed_at)
		SELECT ?, id, ?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?::jsonb, ?, ?, ?, ?
		FROM enrollment.requests WHERE id = ? AND tenant_id = ?
		RETURNING id, tenant_id, created_at, updated_at`, tenantID, row.RequestChildID, row.Origin, row.Status,
		row.ParentNote, row.AdminDecisionNote, encoded[0], encoded[1], encoded[2], row.CareOfferingsEnabledAtCreation,
		row.CreatedByAccountID, row.ReviewedByAccountID, row.ReviewedAt, row.RequestID, tenantID).Scan(ctx, row)
	if err != nil {
		return fmt.Errorf("failed to create enrollment change request: %w", err)
	}
	return nil
}
