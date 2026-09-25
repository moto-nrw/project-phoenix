package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// careOfferingFixtureRecords writes and reads Care Plan's care offerings in
// the enrollment model rows the suites build their fixtures from. It is the
// translation the retained enrollment repository adapter did before #3565
// moved its consumers into the Enrollment owner, kept here as a fixture
// writer over Care Plan's commands and queries; it holds no persistence of
// its own.
type careOfferingFixtureRecords struct{ carePlan careplan.Capability }

func newCareOfferingFixtureRecords(capability careplan.Capability) careOfferingFixtureRecords {
	return careOfferingFixtureRecords{carePlan: capability}
}

func (r careOfferingFixtureRecords) Create(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil {
		return errors.New("CareOffering cannot be nil or zero value")
	}
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fields, err := careOfferingFixtureFields(offering)
	if err != nil {
		return fmt.Errorf("encode care offering: %w", err)
	}
	created, err := r.carePlan.CreateCareOffering(ctx, careplan.CreateCareOffering{CareOfferingFields: fields})
	if err != nil {
		return fmt.Errorf("failed to create care offering: %w", err)
	}
	return applyCareOfferingFixture(offering, created)
}

func (r careOfferingFixtureRecords) FindByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	value, err := r.carePlan.FindCareOffering(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find care offering %d: %w", id, err)
	}
	offering := new(enrollmentModels.CareOffering)
	return offering, applyCareOfferingFixture(offering, value)
}

func (r careOfferingFixtureRecords) Update(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil {
		return errors.New("CareOffering cannot be nil or zero value")
	}
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fields, err := careOfferingFixtureFields(offering)
	if err != nil {
		return fmt.Errorf("encode care offering: %w", err)
	}
	updated, err := r.carePlan.UpdateCareOffering(ctx, careplan.UpdateCareOffering{ID: offering.ID, CareOfferingFields: fields})
	if err != nil {
		return fmt.Errorf("failed to update care offering: %w", err)
	}
	return applyCareOfferingFixture(offering, updated)
}

func (r careOfferingFixtureRecords) Delete(ctx context.Context, id int64) error {
	if err := r.carePlan.DeleteCareOffering(ctx, id); err != nil {
		return fmt.Errorf("failed to delete care offering: %w", err)
	}
	return nil
}

func (r careOfferingFixtureRecords) ReplaceAutoAddTriggers(ctx context.Context, id int64, triggers []int64) error {
	if err := r.carePlan.ReplaceAutoAddTriggers(ctx, id, triggers); err != nil {
		return fmt.Errorf("failed to replace care offering auto triggers: %w", err)
	}
	return nil
}

// ListByTenant lists every offering of the tenant in catalog order.
func (r careOfferingFixtureRecords) ListByTenant(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	values, err := r.carePlan.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderCatalog})
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings: %w", err)
	}
	result := make([]*enrollmentModels.CareOffering, 0, len(values))
	for _, value := range values {
		offering := new(enrollmentModels.CareOffering)
		if err := applyCareOfferingFixture(offering, value); err != nil {
			return nil, err
		}
		result = append(result, offering)
	}
	return result, nil
}

func careOfferingFixtureFields(offering *enrollmentModels.CareOffering) (careplan.CareOfferingFields, error) {
	availabilityRule, err := json.Marshal(offering.AvailabilityRule)
	if err != nil {
		return careplan.CareOfferingFields{}, err
	}
	return careplan.CareOfferingFields{
		PhaseID: offering.PhaseID, ActivityGroupID: offering.ActivityGroupID, Name: offering.Name, Description: offering.Description,
		DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays,
		IncludesHolidayCare: offering.IncludesHolidayCare, IncludesLunch: offering.IncludesLunch,
		Capacity: offering.Capacity, PriceCents: offering.PriceCents, IsActive: offering.IsActive, IsRequired: offering.IsRequired,
		CountsAsCare: offering.CountsAsCare, AutoAddGradeLevels: offering.AutoAddGradeLevels,
		AvailabilityRule: availabilityRule, SortOrder: offering.SortOrder,
		SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule, PickupTimes: offering.PickupTimes,
		AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, Translations: offering.Translations,
	}, nil
}

func applyCareOfferingFixture(target *enrollmentModels.CareOffering, value careplan.CareOffering) error {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.TenantID = value.TenantID
	target.PhaseID = value.PhaseID
	target.ActivityGroupID = value.ActivityGroupID
	target.Name = value.Name
	target.Description = value.Description
	target.DaysOfWeekMode = value.DaysOfWeekMode
	target.AvailableDays = value.AvailableDays
	target.IncludesHolidayCare = value.IncludesHolidayCare
	target.IncludesLunch = value.IncludesLunch
	target.Capacity = value.Capacity
	target.PriceCents = value.PriceCents
	target.IsActive = value.IsActive
	target.IsRequired = value.IsRequired
	target.CountsAsCare = value.CountsAsCare
	target.CountsAsCareSet = true
	target.AutoAddGradeLevels = value.AutoAddGradeLevels
	if len(value.AvailabilityRule) > 0 && string(value.AvailabilityRule) != "null" {
		if err := json.Unmarshal(value.AvailabilityRule, &target.AvailabilityRule); err != nil {
			return fmt.Errorf("decode care offering availability rule: %w", err)
		}
	}
	target.SortOrder = value.SortOrder
	target.SelectionGroup = value.SelectionGroup
	target.SelectionRule = value.SelectionRule
	target.PickupTimes, target.Translations = value.PickupTimes, value.Translations
	target.AutoAddTriggerOfferingIDs = value.AutoAddTriggerOfferingIDs
	return nil
}

// pendingOfferingChangeForStudent loads the student's oldest pending offering
// change request, or nil when there is none.
func pendingOfferingChangeForStudent(ctx context.Context, carePlan careplan.Capability, studentID int64) (*careplan.OfferingChangeRequest, error) {
	rows, err := carePlan.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{
		StudentID: studentID, Statuses: []string{enrollmentModels.OfferingChangeStatusPending}, Order: careplan.ChangeOrderCreated,
	})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}
