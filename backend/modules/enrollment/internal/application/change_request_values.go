package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The owner stores a change request's snapshots as JSON; the review decodes
// them into maps and encodes them back for the public contract.

func changeRequestFromOwner(value *enrollment.ChangeRequest) (*ChangeRequest, error) {
	if value == nil {
		return nil, nil
	}
	row := &ChangeRequest{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		RequestID: value.RequestID, RequestChildID: value.RequestChildID, Origin: value.Origin, Status: value.Status,
		ParentNote: value.ParentNote, AdminDecisionNote: value.AdminDecisionNote,
		CareOfferingsEnabledAtCreation: value.CareOfferingsEnabledAtCreation, CreatedByAccountID: value.CreatedByAccountID,
		ReviewedByAccountID: value.ReviewedByAccountID, ReviewedAt: value.ReviewedAt,
	}
	for _, snapshot := range []struct {
		name string
		raw  json.RawMessage
		into *map[string]any
	}{
		{"BaseSnapshot", value.BaseSnapshot, &row.BaseSnapshot},
		{"ProposedSnapshot", value.ProposedSnapshot, &row.ProposedSnapshot},
		{"Diff", value.Diff, &row.Diff},
	} {
		if len(snapshot.raw) == 0 {
			continue
		}
		if err := json.Unmarshal(snapshot.raw, snapshot.into); err != nil {
			return nil, fmt.Errorf("decode change request %s: %w", snapshot.name, err)
		}
	}
	return row, nil
}

// changeRequestsFromOwner converts an owner read, keeping its error and the
// nil-versus-empty distinction of its slice.
func changeRequestsFromOwner(values []*enrollment.ChangeRequest, err error) ([]*ChangeRequest, error) {
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}
	rows := make([]*ChangeRequest, 0, len(values))
	for _, value := range values {
		row, err := changeRequestFromOwner(value)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// changeRequestToOwner encodes a decoded change request; an absent snapshot
// is stored as an empty object.
func changeRequestToOwner(row *ChangeRequest) (*enrollment.ChangeRequest, error) {
	encoded := make([]json.RawMessage, 0, 3)
	for _, snapshot := range []map[string]any{row.BaseSnapshot, row.ProposedSnapshot, row.Diff} {
		if snapshot == nil {
			snapshot = map[string]any{}
		}
		data, err := json.Marshal(snapshot)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, data)
	}
	return &enrollment.ChangeRequest{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		RequestID: row.RequestID, RequestChildID: row.RequestChildID, Origin: row.Origin, Status: row.Status,
		ParentNote: row.ParentNote, AdminDecisionNote: row.AdminDecisionNote,
		BaseSnapshot: encoded[0], ProposedSnapshot: encoded[1], Diff: encoded[2],
		CareOfferingsEnabledAtCreation: row.CareOfferingsEnabledAtCreation, CreatedByAccountID: row.CreatedByAccountID,
		ReviewedByAccountID: row.ReviewedByAccountID, ReviewedAt: row.ReviewedAt,
	}, nil
}

// createChangeRequest stores a change request and refreshes the row with the
// generated metadata.
func createChangeRequest(ctx context.Context, owner interface {
	InsertChangeRequest(context.Context, *enrollment.ChangeRequest) error
}, row *ChangeRequest) error {
	value, err := changeRequestToOwner(row)
	if err != nil {
		return fmt.Errorf("failed to create enrollment change request: %w", err)
	}
	if err := owner.InsertChangeRequest(ctx, value); err != nil {
		return err
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	row.RequestID, row.RequestChildID, row.Origin, row.Status = value.RequestID, value.RequestChildID, value.Origin, value.Status
	row.ParentNote, row.AdminDecisionNote = value.ParentNote, value.AdminDecisionNote
	for _, snapshot := range []*map[string]any{&row.BaseSnapshot, &row.ProposedSnapshot, &row.Diff} {
		if *snapshot == nil {
			*snapshot = map[string]any{}
		}
	}
	row.CareOfferingsEnabledAtCreation = value.CareOfferingsEnabledAtCreation
	row.CreatedByAccountID, row.ReviewedByAccountID, row.ReviewedAt = value.CreatedByAccountID, value.ReviewedByAccountID, value.ReviewedAt
	return nil
}

func (c *changeRequestCase) public() (*enrollment.ChangeRequestCase, error) {
	changeRequest, err := changeRequestToOwner(c.ChangeRequest)
	if err != nil {
		return nil, err
	}
	request, err := requestInput(c.Request)
	if err != nil {
		return nil, err
	}
	children, err := childInputs(c.Children)
	if err != nil {
		return nil, err
	}
	return &enrollment.ChangeRequestCase{
		ChangeRequest: changeRequest, Request: request, Children: children, Messages: c.Messages, Phase: c.Phase,
	}, nil
}

// publicCase encodes one case and marks its error with Enrollment's values.
func publicCase(value *changeRequestCase, err error) (*enrollment.ChangeRequestCase, error) {
	if err != nil {
		return nil, publicError(err)
	}
	return value.public()
}

func publicCases(values []*changeRequestCase, err error) ([]*enrollment.ChangeRequestCase, error) {
	if err != nil {
		return nil, publicError(err)
	}
	out := make([]*enrollment.ChangeRequestCase, 0, len(values))
	for _, value := range values {
		converted, err := value.public()
		if err != nil {
			return nil, err
		}
		out = append(out, converted)
	}
	return out, nil
}
