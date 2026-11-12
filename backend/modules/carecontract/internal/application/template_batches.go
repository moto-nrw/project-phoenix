package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
)

// PrepareTemplateBatch validates the complete module set in the caller's
// school unit. Its returned command joins the later financial Träger unit.
func (in *Intake) PrepareTemplateBatch(ctx context.Context, reviewer carecontract.Reviewer, schoolID int64, inputs []carecontract.ModuleTemplateInput) (carecontract.PreparedTemplates, error) {
	if err := checkReviewer(reviewer); err != nil {
		return nil, err
	}
	ids, normalized, err := normalizeTemplateBatch(inputs)
	if err != nil {
		return nil, err
	}
	if err := in.checkTemplateBatchOfferings(ctx, schoolID, ids); err != nil {
		return nil, err
	}
	return func(ctx context.Context) error { return in.saveTemplateBatch(ctx, reviewer, schoolID, normalized) }, nil
}

func normalizeTemplateBatch(inputs []carecontract.ModuleTemplateInput) ([]int64, []carecontract.ModuleTemplateInput, error) {
	ids := make([]int64, 0, len(inputs))
	normalized := make([]carecontract.ModuleTemplateInput, 0, len(inputs))
	seen := map[int64]bool{}
	for _, input := range inputs {
		if input.CareOfferingID <= 0 || seen[input.CareOfferingID] {
			return nil, nil, &carecontract.ValidationError{Fields: map[string]string{"care_offering_id": "invalid"}}
		}
		seen[input.CareOfferingID] = true
		input.Input = normalizeTemplate(input.Input)
		if fields := validateTemplate(input.Input); len(fields) > 0 {
			return nil, nil, &carecontract.ValidationError{Fields: fields}
		}
		ids = append(ids, input.CareOfferingID)
		if input.Input.TermMonths != nil {
			months := *input.Input.TermMonths
			input.Input.TermMonths = &months
		}
		normalized = append(normalized, input)
	}
	return ids, normalized, nil
}

func (in *Intake) checkTemplateBatchOfferings(ctx context.Context, schoolID int64, ids []int64) error {
	if len(ids) > 0 {
		err := in.runtime.WithinSchool(ctx, schoolID, func(ctx context.Context) error {
			found, err := in.bookings.Offerings(ctx, ids)
			if err != nil {
				return err
			}
			if len(found) != len(ids) {
				return carecontract.ErrOfferingNotFound
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (in *Intake) saveTemplateBatch(ctx context.Context, reviewer carecontract.Reviewer, schoolID int64, normalized []carecontract.ModuleTemplateInput) error {

	if len(normalized) == 0 {
		return nil
	}
	if _, err := in.schoolsOf(ctx, reviewer.OrganizationID, schoolID); err != nil {
		return err
	}
	rows, err := in.store.UpsertTemplates(ctx, reviewer.OrganizationID, reviewer.AccountID, schoolID, normalized)
	if err != nil {
		return err
	}
	events := make([]ports.Event, 0, len(normalized))
	for _, row := range rows {
		school := schoolID
		events = append(events, ports.Event{SchoolID: &school, AccountID: reviewer.AccountID, Action: ports.ActionTemplate,
			Details: map[string]any{"care_offering_id": row.OfferingID, "template_id": row.Template.ID}})
	}
	return in.store.InsertEvents(ctx, reviewer.OrganizationID, events)
}
