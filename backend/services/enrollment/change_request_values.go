package enrollment

import (
	"context"
	"encoding/json"
	"fmt"

	legacy "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func createChangeRequest(ctx context.Context, owner ChangeRequestIntakeRequests, row *legacy.ChangeRequest) error {
	encoded := make([]json.RawMessage, 0, 3)
	for _, snapshot := range []map[string]any{row.BaseSnapshot, row.ProposedSnapshot, row.Diff} {
		if snapshot == nil {
			snapshot = map[string]any{}
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			return fmt.Errorf("failed to create enrollment change request: %w", err)
		}
		encoded = append(encoded, data)
	}
	value := &capability.ChangeRequest{
		ID:                             row.ID,
		TenantID:                       row.TenantID,
		CreatedAt:                      row.CreatedAt,
		UpdatedAt:                      row.UpdatedAt,
		RequestID:                      row.RequestID,
		RequestChildID:                 row.RequestChildID,
		Origin:                         row.Origin,
		Status:                         row.Status,
		ParentNote:                     row.ParentNote,
		AdminDecisionNote:              row.AdminDecisionNote,
		BaseSnapshot:                   encoded[0],
		ProposedSnapshot:               encoded[1],
		Diff:                           encoded[2],
		CareOfferingsEnabledAtCreation: row.CareOfferingsEnabledAtCreation,
		CreatedByAccountID:             row.CreatedByAccountID,
		ReviewedByAccountID:            row.ReviewedByAccountID,
		ReviewedAt:                     row.ReviewedAt,
	}
	if err := owner.InsertChangeRequest(ctx, value); err != nil {
		return err
	}
	row.ID = value.ID
	row.TenantID = value.TenantID
	row.CreatedAt = value.CreatedAt
	row.UpdatedAt = value.UpdatedAt
	row.RequestID = value.RequestID
	row.RequestChildID = value.RequestChildID
	row.Origin = value.Origin
	row.Status = value.Status
	row.ParentNote = value.ParentNote
	row.AdminDecisionNote = value.AdminDecisionNote
	if row.BaseSnapshot == nil {
		row.BaseSnapshot = map[string]any{}
	}
	if row.ProposedSnapshot == nil {
		row.ProposedSnapshot = map[string]any{}
	}
	if row.Diff == nil {
		row.Diff = map[string]any{}
	}
	row.CareOfferingsEnabledAtCreation = value.CareOfferingsEnabledAtCreation
	row.CreatedByAccountID = value.CreatedByAccountID
	row.ReviewedByAccountID = value.ReviewedByAccountID
	row.ReviewedAt = value.ReviewedAt
	return nil
}
