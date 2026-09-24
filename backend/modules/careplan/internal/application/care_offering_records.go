package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The helpers below are the catalog's access to its own rows. They keep the
// semantics the enrollment-model repository gave the catalog before it moved
// (#3559): every write re-runs the field rules, and each failure carries the
// operation it failed in.

func phaseFilter(phaseID int64) careplan.CareOfferingFilter {
	return careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog}
}

func (c *CareOfferingCatalog) listRecords(ctx context.Context, filter careplan.CareOfferingFilter, message string) ([]careplan.CareOffering, error) {
	if filter.IDs != nil && len(filter.IDs) == 0 {
		return []careplan.CareOffering{}, nil
	}
	if filter.ActivityGroupIDs != nil && len(filter.ActivityGroupIDs) == 0 {
		return []careplan.CareOffering{}, nil
	}
	offerings, err := c.deps.Records.ListCareOfferings(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", message, err)
	}
	return offerings, nil
}

// listValidated lists offerings and refuses a persisted availability rule
// that no longer validates, so a corrupt row never reaches a reader.
func (c *CareOfferingCatalog) listValidated(ctx context.Context, filter careplan.CareOfferingFilter, message string) ([]careplan.CareOffering, error) {
	offerings, err := c.listRecords(ctx, filter, message)
	if err != nil {
		return nil, err
	}
	for i := range offerings {
		if err := normalizeLoadedAvailabilityRule(&offerings[i]); err != nil {
			return nil, err
		}
	}
	return offerings, nil
}

func (c *CareOfferingCatalog) listByIDs(ctx context.Context, ids []int64) ([]careplan.CareOffering, error) {
	if ids == nil {
		ids = []int64{}
	}
	return c.listRecords(ctx, careplan.CareOfferingFilter{IDs: ids, Order: careplan.OfferingOrderCatalog}, "failed to list care offerings by ids")
}

func (c *CareOfferingCatalog) listByActivityGroupIDs(ctx context.Context, ids []int64) ([]careplan.CareOffering, error) {
	if ids == nil {
		ids = []int64{}
	}
	return c.listRecords(ctx, careplan.CareOfferingFilter{ActivityGroupIDs: ids, Order: careplan.OfferingOrderID}, "failed to list care offerings by activity groups")
}

func (c *CareOfferingCatalog) listTenantRecords(ctx context.Context) ([]careplan.CareOffering, error) {
	return c.listRecords(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderCatalog}, "failed to list care offerings")
}

// findRecord returns careplan.ErrCareOfferingNotFound (wrapped) for a
// missing row.
func (c *CareOfferingCatalog) findRecord(ctx context.Context, id int64) (careplan.CareOffering, error) {
	offering, err := c.deps.Records.FindCareOffering(ctx, id)
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return careplan.CareOffering{}, fmt.Errorf("care offering %d not found: %w", id, err)
	}
	if err != nil {
		return careplan.CareOffering{}, fmt.Errorf("failed to find care offering: %w", err)
	}
	return offering, nil
}

func (c *CareOfferingCatalog) createRecord(ctx context.Context, offering careplan.CareOffering) (careplan.CareOffering, error) {
	if err := validateOfferingFields(&offering); err != nil {
		return careplan.CareOffering{}, fmt.Errorf("validation failed: %w", err)
	}
	created, err := c.deps.Records.CreateCareOffering(ctx, careplan.CreateCareOffering{CareOfferingFields: offeringFields(offering)})
	if err != nil {
		return careplan.CareOffering{}, fmt.Errorf("failed to create care offering: %w", err)
	}
	return created, nil
}

func (c *CareOfferingCatalog) updateRecord(ctx context.Context, offering careplan.CareOffering) (careplan.CareOffering, error) {
	if err := validateOfferingFields(&offering); err != nil {
		return careplan.CareOffering{}, fmt.Errorf("validation failed: %w", err)
	}
	updated, err := c.deps.Records.UpdateCareOffering(ctx, careplan.UpdateCareOffering{ID: offering.ID, CareOfferingFields: offeringFields(offering)})
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return careplan.CareOffering{}, fmt.Errorf("care offering %d not found", offering.ID)
	}
	if err != nil {
		return careplan.CareOffering{}, fmt.Errorf("failed to update care offering: %w", err)
	}
	return updated, nil
}

func (c *CareOfferingCatalog) deleteRecord(ctx context.Context, id int64) error {
	err := c.deps.Records.DeleteCareOffering(ctx, id)
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return fmt.Errorf("care offering %d not found", id)
	}
	if err != nil {
		return fmt.Errorf("failed to delete care offering: %w", err)
	}
	return nil
}

func (c *CareOfferingCatalog) replaceTriggers(ctx context.Context, offeringID int64, triggers []int64) error {
	if err := c.deps.Records.ReplaceAutoAddTriggers(ctx, offeringID, triggers); err != nil {
		return fmt.Errorf("failed to replace care offering auto triggers: %w", err)
	}
	return nil
}

func offeringFields(offering careplan.CareOffering) careplan.CareOfferingFields {
	return careplan.CareOfferingFields{
		PhaseID: offering.PhaseID, ActivityGroupID: offering.ActivityGroupID, Name: offering.Name, Description: offering.Description,
		DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays,
		IncludesHolidayCare: offering.IncludesHolidayCare, IncludesLunch: offering.IncludesLunch,
		Capacity: offering.Capacity, PriceCents: offering.PriceCents, IsActive: offering.IsActive, IsRequired: offering.IsRequired,
		CountsAsCare: offering.CountsAsCare, AutoAddGradeLevels: offering.AutoAddGradeLevels,
		AvailabilityRule: offering.AvailabilityRule, SortOrder: offering.SortOrder,
		SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule, PickupTimes: offering.PickupTimes,
		Translations: offering.Translations, AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs,
	}
}
