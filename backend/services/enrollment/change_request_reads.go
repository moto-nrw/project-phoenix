package enrollment

import (
	"context"
	"encoding/json"
	"fmt"

	legacy "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func legacyChangeRequest(value *capability.ChangeRequest) (*legacy.ChangeRequest, error) {
	if value == nil {
		return nil, nil
	}
	row := new(legacy.ChangeRequest)
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
	row.CareOfferingsEnabledAtCreation = value.CareOfferingsEnabledAtCreation
	row.CreatedByAccountID = value.CreatedByAccountID
	row.ReviewedByAccountID = value.ReviewedByAccountID
	row.ReviewedAt = value.ReviewedAt
	if len(value.BaseSnapshot) > 0 {
		if err := json.Unmarshal(value.BaseSnapshot, &row.BaseSnapshot); err != nil {
			return nil, fmt.Errorf("decode change request BaseSnapshot: %w", err)
		}
	}
	if len(value.ProposedSnapshot) > 0 {
		if err := json.Unmarshal(value.ProposedSnapshot, &row.ProposedSnapshot); err != nil {
			return nil, fmt.Errorf("decode change request ProposedSnapshot: %w", err)
		}
	}
	if len(value.Diff) > 0 {
		if err := json.Unmarshal(value.Diff, &row.Diff); err != nil {
			return nil, fmt.Errorf("decode change request Diff: %w", err)
		}
	}
	return row, nil
}
func readChangeRequestByID(ctx context.Context, owner ChangeRequestIntakeRequests, id int64) (*legacy.ChangeRequest, error) {
	values, err := owner.ChangeRequestByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return legacyChangeRequest(values)
}
func readChangeRequestByIDForUpdate(ctx context.Context, owner ChangeRequestIntakeRequests, id int64) (*legacy.ChangeRequest, error) {
	values, err := owner.ChangeRequestByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return legacyChangeRequest(values)
}
func readChangeRequestsForRequest(ctx context.Context, owner ChangeRequestIntakeRequests, requestID int64) ([]*legacy.ChangeRequest, error) {
	values, err := owner.ChangeRequestsForRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	rows := make([]*legacy.ChangeRequest, 0, len(values))
	for _, value := range values {
		row, err := legacyChangeRequest(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func readOpenChangeRequestsForRequestForUpdate(ctx context.Context, owner ChangeRequestIntakeRequests, requestID int64) ([]*legacy.ChangeRequest, error) {
	values, err := owner.OpenChangeRequestsForRequestForUpdate(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	rows := make([]*legacy.ChangeRequest, 0, len(values))
	for _, value := range values {
		row, err := legacyChangeRequest(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func readListChangeRequests(ctx context.Context, owner ChangeRequestIntakeRequests, filters capability.ChangeRequestListFilters) ([]*legacy.ChangeRequest, error) {
	values, err := owner.ListChangeRequests(ctx, filters)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	rows := make([]*legacy.ChangeRequest, 0, len(values))
	for _, value := range values {
		row, err := legacyChangeRequest(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func readChangeRequestsForReview(ctx context.Context, owner ChangeRequestIntakeRequests, filters capability.ChangeRequestReviewFilters) ([]*legacy.ChangeRequest, error) {
	values, err := owner.ChangeRequestsForReview(ctx, filters)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	rows := make([]*legacy.ChangeRequest, 0, len(values))
	for _, value := range values {
		row, err := legacyChangeRequest(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
