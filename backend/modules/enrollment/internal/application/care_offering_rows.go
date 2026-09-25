package application

import (
	"context"
	"encoding/json"
	"fmt"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Care Plan owns the care-offering catalog (#3559). Enrollment's flows and
// routes still speak the enrollment offering rows; the translations below
// serve Care Plan's values in those rows. They hold no rule of their own.

// CarePlanOfferings is the slice of Care Plan's capability the offering
// reads list through.
type CarePlanOfferings interface {
	ListCareOfferings(ctx context.Context, filter careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
}

// CareOfferingRecords reads Care Plan's offerings in enrollment rows for the
// intake, the decision flow, the capacity gate and the reports.
type CareOfferingRecords struct {
	carePlan CarePlanOfferings
}

// NewCareOfferingRecords binds the reads to Care Plan.
func NewCareOfferingRecords(carePlan CarePlanOfferings) *CareOfferingRecords {
	return &CareOfferingRecords{carePlan: carePlan}
}

// ListByPhase lists every offering of a phase in catalog order.
func (r *CareOfferingRecords) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return r.list(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog}, "failed to list care offerings by phase")
}

// ListActiveByPhase lists the active offerings of a phase in catalog order.
func (r *CareOfferingRecords) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return r.list(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, ActiveOnly: true, Order: careplan.OfferingOrderCatalog}, "failed to list active offerings by phase")
}

// ListByIDs lists the offerings with the given ids in catalog order.
func (r *CareOfferingRecords) ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollmentModels.CareOffering{}, nil
	}
	return r.list(ctx, careplan.CareOfferingFilter{IDs: ids, Order: careplan.OfferingOrderCatalog}, "failed to list care offerings by ids")
}

// ListByIDsForUpdate locks the offerings with the given ids by ascending id.
func (r *CareOfferingRecords) ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollmentModels.CareOffering{}, nil
	}
	return r.list(ctx, careplan.CareOfferingFilter{IDs: ids, LockForUpdate: true, Order: careplan.OfferingOrderID}, "failed to lock care offerings by ids")
}

func (r *CareOfferingRecords) list(ctx context.Context, filter careplan.CareOfferingFilter, message string) ([]*enrollmentModels.CareOffering, error) {
	values, err := r.carePlan.ListCareOfferings(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", message, err)
	}
	result := make([]*enrollmentModels.CareOffering, 0, len(values))
	for _, value := range values {
		offering, convertErr := careOfferingRow(value)
		if convertErr != nil {
			return nil, fmt.Errorf("%s: %w", message, convertErr)
		}
		result = append(result, offering)
	}
	return result, nil
}

// CareOfferingCatalogAdministration is Care Plan's catalog administration the
// rows translate.
type CareOfferingCatalogAdministration = careplan.CareOfferingCatalog

// CareOfferingRows serves Care Plan's catalog administration to the
// enrollment routes in enrollment rows. Every operation runs through the
// same translation: Care Plan's values become rows, and Care Plan's refusals
// come back marked with Enrollment's public values.
type CareOfferingRows struct {
	catalog careplan.CareOfferingCatalog
}

// NewCareOfferingRows binds the rows to the catalog.
func NewCareOfferingRows(catalog careplan.CareOfferingCatalog) *CareOfferingRows {
	return &CareOfferingRows{catalog: catalog}
}

type (
	catalogListing   func(careplan.CareOfferingCatalog) ([]careplan.CareOffering, error)
	catalogOperation func(careplan.CareOfferingCatalog) (careplan.CareOffering, error)
)

// List lists the tenant's catalog.
func (r *CareOfferingRows) List(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	return r.rows(func(catalog careplan.CareOfferingCatalog) ([]careplan.CareOffering, error) {
		return catalog.ListCatalog(ctx)
	})
}

// ListByPhase lists the catalog of one phase.
func (r *CareOfferingRows) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return r.rows(func(catalog careplan.CareOfferingCatalog) ([]careplan.CareOffering, error) {
		return catalog.ListByPhase(ctx, phaseID)
	})
}

// GetByID loads one offering.
func (r *CareOfferingRows) GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	return r.row(func(catalog careplan.CareOfferingCatalog) (careplan.CareOffering, error) {
		return catalog.FindOffering(ctx, id)
	})
}

// Create saves a new offering and returns the stored row.
func (r *CareOfferingRows) Create(ctx context.Context, row *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	return row, r.save(row, func(catalog careplan.CareOfferingCatalog, offering careplan.CareOffering) (careplan.CareOffering, error) {
		return catalog.CreateOffering(ctx, offering)
	})
}

// Update saves the offering and refreshes row with the stored state.
func (r *CareOfferingRows) Update(ctx context.Context, row *enrollmentModels.CareOffering) error {
	return r.save(row, func(catalog careplan.CareOfferingCatalog, offering careplan.CareOffering) (careplan.CareOffering, error) {
		return catalog.UpdateOffering(ctx, offering)
	})
}

// Delete deletes an offering.
func (r *CareOfferingRows) Delete(ctx context.Context, id int64) error {
	_, err := r.row(func(catalog careplan.CareOfferingCatalog) (careplan.CareOffering, error) {
		return careplan.CareOffering{}, catalog.DeleteOffering(ctx, id)
	})
	return err
}

// Clone copies an offering into a target phase.
func (r *CareOfferingRows) Clone(ctx context.Context, sourceID, targetPhaseID int64) (*enrollmentModels.CareOffering, error) {
	return r.row(func(catalog careplan.CareOfferingCatalog) (careplan.CareOffering, error) {
		return catalog.Clone(ctx, sourceID, targetPhaseID)
	})
}

// ListBookingStats reports how full each offering of the phase is.
func (r *CareOfferingRows) ListBookingStats(ctx context.Context, phaseID int64) ([]enrollment.CareOfferingBookingStat, error) {
	return r.bookingStats(func(catalog careplan.CareOfferingCatalog) ([]careplan.CareOfferingBookingStat, error) {
		return catalog.ListBookingStats(ctx, phaseID)
	})
}

func (r *CareOfferingRows) rows(list catalogListing) ([]*enrollmentModels.CareOffering, error) {
	offerings, err := careOfferingRowsOf(list(r.catalog))
	return offerings, publicError(err)
}

func (r *CareOfferingRows) row(operation catalogOperation) (*enrollmentModels.CareOffering, error) {
	offering, err := operation(r.catalog)
	if err != nil {
		return nil, publicError(err)
	}
	return careOfferingRow(offering)
}

func (r *CareOfferingRows) save(row *enrollmentModels.CareOffering, write func(careplan.CareOfferingCatalog, careplan.CareOffering) (careplan.CareOffering, error)) error {
	offering, err := careOfferingFromRow(row)
	if err != nil {
		return err
	}
	stored, err := write(r.catalog, offering)
	if err != nil {
		return publicError(err)
	}
	return applyCareOfferingToRow(row, stored)
}

func (r *CareOfferingRows) bookingStats(list func(careplan.CareOfferingCatalog) ([]careplan.CareOfferingBookingStat, error)) ([]enrollment.CareOfferingBookingStat, error) {
	stats, err := list(r.catalog)
	if err != nil {
		return nil, publicError(err)
	}
	result := make([]enrollment.CareOfferingBookingStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, enrollment.CareOfferingBookingStat{
			OfferingID: stat.OfferingID, Capacity: stat.Capacity, Booked: stat.Booked,
			GradeLevels: stat.GradeLevels, UnknownGradeCount: stat.UnknownGradeCount,
		})
	}
	return result, nil
}

func careOfferingRowsOf(offerings []careplan.CareOffering, err error) ([]*enrollmentModels.CareOffering, error) {
	if err != nil {
		return nil, err
	}
	rows := make([]*enrollmentModels.CareOffering, 0, len(offerings))
	for _, offering := range offerings {
		row, convertErr := careOfferingRow(offering)
		if convertErr != nil {
			return nil, convertErr
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// careOfferingFromRow translates an enrollment row into the owner value. A
// row that never set counts_as_care counts as care, the column default.
func careOfferingFromRow(row *enrollmentModels.CareOffering) (careplan.CareOffering, error) {
	availabilityRule, err := marshalOptional(row.AvailabilityRule)
	if err != nil {
		return careplan.CareOffering{}, err
	}
	return careplan.CareOffering{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PhaseID: row.PhaseID, ActivityGroupID: row.ActivityGroupID, Name: row.Name, Description: row.Description,
		DaysOfWeekMode: row.DaysOfWeekMode, AvailableDays: row.AvailableDays,
		IncludesHolidayCare: row.IncludesHolidayCare, IncludesLunch: row.IncludesLunch,
		Capacity: row.Capacity, PriceCents: row.PriceCents, IsActive: row.IsActive, IsRequired: row.IsRequired,
		CountsAsCare: row.CountsAsCare || !row.CountsAsCareSet, AutoAddGradeLevels: row.AutoAddGradeLevels,
		AvailabilityRule: availabilityRule, SortOrder: row.SortOrder,
		SelectionGroup: row.SelectionGroup, SelectionRule: row.SelectionRule, PickupTimes: row.PickupTimes,
		Translations: row.Translations, AutoAddTriggerOfferingIDs: row.AutoAddTriggerOfferingIDs,
	}, nil
}

func careOfferingRow(value careplan.CareOffering) (*enrollmentModels.CareOffering, error) {
	row := new(enrollmentModels.CareOffering)
	return row, applyCareOfferingToRow(row, value)
}

func applyCareOfferingToRow(target *enrollmentModels.CareOffering, value careplan.CareOffering) error {
	target.ID, target.CreatedAt, target.UpdatedAt, target.TenantID = value.ID, value.CreatedAt, value.UpdatedAt, value.TenantID
	target.PhaseID, target.ActivityGroupID = value.PhaseID, value.ActivityGroupID
	target.Name, target.Description = value.Name, value.Description
	target.DaysOfWeekMode, target.AvailableDays = value.DaysOfWeekMode, value.AvailableDays
	target.IncludesHolidayCare, target.IncludesLunch = value.IncludesHolidayCare, value.IncludesLunch
	target.Capacity, target.PriceCents = value.Capacity, value.PriceCents
	target.IsActive, target.IsRequired = value.IsActive, value.IsRequired
	target.CountsAsCare, target.CountsAsCareSet = value.CountsAsCare, true
	target.AutoAddGradeLevels = value.AutoAddGradeLevels
	if err := unmarshalOptional(value.AvailabilityRule, &target.AvailabilityRule); err != nil {
		return fmt.Errorf("decode care offering availability rule: %w", err)
	}
	target.SortOrder, target.SelectionGroup, target.SelectionRule = value.SortOrder, value.SelectionGroup, value.SelectionRule
	target.PickupTimes, target.Translations = value.PickupTimes, value.Translations
	target.AutoAddTriggerOfferingIDs = value.AutoAddTriggerOfferingIDs
	return nil
}

// publicCareOfferings describes enrollment rows in the public offering value
// of the form loads.
func publicCareOfferings(rows []*enrollmentModels.CareOffering) []*enrollment.CareOffering {
	out := make([]*enrollment.CareOffering, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		rule, err := marshalOptional(row.AvailabilityRule)
		if err != nil {
			rule = nil
		}
		out = append(out, &enrollment.CareOffering{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			PhaseID: row.PhaseID, ActivityGroupID: row.ActivityGroupID, Name: row.Name, Description: row.Description,
			DaysOfWeekMode: row.DaysOfWeekMode, AvailableDays: row.AvailableDays,
			IncludesHolidayCare: row.IncludesHolidayCare, IncludesLunch: row.IncludesLunch,
			Capacity: row.Capacity, PriceCents: row.PriceCents, IsActive: row.IsActive, IsRequired: row.IsRequired,
			CountsAsCare: row.CountsAsCare, AutoAddGradeLevels: row.AutoAddGradeLevels, AvailabilityRule: rule,
			SortOrder: row.SortOrder, SelectionGroup: row.SelectionGroup, SelectionRule: row.SelectionRule,
			PickupTimes: row.PickupTimes, Translations: row.Translations, AutoAddTriggerOfferingIDs: row.AutoAddTriggerOfferingIDs,
		})
	}
	return out
}

// marshalOptional encodes value; only an untyped nil stays absent.
func marshalOptional(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func unmarshalOptional(data json.RawMessage, target any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}
