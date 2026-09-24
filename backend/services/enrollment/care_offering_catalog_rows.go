package enrollment

import (
	"context"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// The care-offering catalog moved to Care Plan (#3559). The enrollment
// routes and services still speak enrollment rows, so this file keeps their
// error names pointed at the owner's values and serves the owner catalog in
// rows. It holds no catalog rule of its own.
var (
	// ErrCareOfferingNotFound is the Care Plan owner value.
	ErrCareOfferingNotFound = careplan.ErrCareOfferingNotFound
	// ErrCareOfferingInvalid is the Care Plan owner value: a catalog or
	// timetable-link configuration an administrator can correct.
	ErrCareOfferingInvalid = careplan.ErrCareOfferingConfigInvalid
	// ErrCareOfferingTemplatePeriodMismatch is the Care Plan owner value.
	ErrCareOfferingTemplatePeriodMismatch = careplan.ErrCareOfferingTemplatePeriodMismatch
	// ErrCareOfferingGroupRuleConflict is the Care Plan owner value.
	ErrCareOfferingGroupRuleConflict = careplan.ErrCareOfferingGroupRuleConflict
	// ErrCareOfferingDaysRequired is the Care Plan owner value.
	ErrCareOfferingDaysRequired = careplan.ErrCareOfferingDaysRequired
	// ErrCareOfferingPickupTimesRequired is the Care Plan owner value.
	ErrCareOfferingPickupTimesRequired = careplan.ErrCareOfferingPickupTimesRequired
)

// CareOfferingBookingStat is one offering's admin-facing booking summary as
// the enrollment routes render it; see careplan.CareOfferingBookingStat.
type CareOfferingBookingStat struct {
	OfferingID        int64
	Capacity          *int
	Booked            int
	GradeLevels       map[int]int
	UnknownGradeCount int
}

// CareOfferingRows serves the Care Plan catalog in enrollment rows.
type CareOfferingRows struct {
	catalog careplan.CareOfferingCatalog
}

// NewCareOfferingRows binds the rows to the owner catalog.
func NewCareOfferingRows(catalog careplan.CareOfferingCatalog) CareOfferingRows {
	return CareOfferingRows{catalog: catalog}
}

func (r CareOfferingRows) List(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	return careOfferingRowsOf(r.catalog.ListCatalog(ctx))
}

func (r CareOfferingRows) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return careOfferingRowsOf(r.catalog.ListByPhase(ctx, phaseID))
}

func (r CareOfferingRows) GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	offering, err := r.catalog.FindOffering(ctx, id)
	if err != nil {
		return nil, err
	}
	return careOfferingToLegacy(offering)
}

// Create saves a new offering and returns the stored row.
func (r CareOfferingRows) Create(ctx context.Context, row *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	offering, err := careOfferingFromRow(row)
	if err != nil {
		return nil, err
	}
	created, err := r.catalog.CreateOffering(ctx, offering)
	if err != nil {
		return nil, err
	}
	return row, applyCareOfferingToLegacy(row, created)
}

// Update saves the offering and refreshes row with the stored state.
func (r CareOfferingRows) Update(ctx context.Context, row *enrollmentModels.CareOffering) error {
	offering, err := careOfferingFromRow(row)
	if err != nil {
		return err
	}
	updated, err := r.catalog.UpdateOffering(ctx, offering)
	if err != nil {
		return err
	}
	return applyCareOfferingToLegacy(row, updated)
}

func (r CareOfferingRows) Delete(ctx context.Context, id int64) error {
	return r.catalog.DeleteOffering(ctx, id)
}

func (r CareOfferingRows) Clone(ctx context.Context, sourceID, targetPhaseID int64) (*enrollmentModels.CareOffering, error) {
	clone, err := r.catalog.Clone(ctx, sourceID, targetPhaseID)
	if err != nil {
		return nil, err
	}
	return careOfferingToLegacy(clone)
}

func (r CareOfferingRows) ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error) {
	stats, err := r.catalog.ListBookingStats(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	result := make([]CareOfferingBookingStat, 0, len(stats))
	for _, stat := range stats {
		result = append(result, CareOfferingBookingStat{
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
		row, convertErr := careOfferingToLegacy(offering)
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
	countsAsCare := row.CountsAsCare || !row.CountsAsCareSet
	fields, err := careOfferingFieldsFromLegacy(row)
	if err != nil {
		return careplan.CareOffering{}, err
	}
	return careplan.CareOffering{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PhaseID: fields.PhaseID, ActivityGroupID: fields.ActivityGroupID, Name: fields.Name, Description: fields.Description,
		DaysOfWeekMode: fields.DaysOfWeekMode, AvailableDays: fields.AvailableDays,
		IncludesHolidayCare: fields.IncludesHolidayCare, IncludesLunch: fields.IncludesLunch,
		Capacity: fields.Capacity, PriceCents: fields.PriceCents, IsActive: fields.IsActive, IsRequired: fields.IsRequired,
		CountsAsCare: countsAsCare, AutoAddGradeLevels: fields.AutoAddGradeLevels,
		AvailabilityRule: fields.AvailabilityRule, SortOrder: fields.SortOrder,
		SelectionGroup: fields.SelectionGroup, SelectionRule: fields.SelectionRule, PickupTimes: fields.PickupTimes,
		Translations: fields.Translations, AutoAddTriggerOfferingIDs: fields.AutoAddTriggerOfferingIDs,
	}, nil
}
