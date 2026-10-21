package application

import (
	context "context"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// CareOfferingValues serves Care Plan's catalog administration to the
// enrollment routes in Enrollment's public offering values (#2734). Every
// operation runs through the row translation of CareOfferingRows, so the
// stored columns and the marked refusals are those of the rows. Delete and
// ListBookingStats need no value translation and are served by the rows.
type CareOfferingValues struct {
	*CareOfferingRows
}

// List lists the tenant's catalog.
func (v *CareOfferingValues) List(ctx context.Context) ([]*enrollment.CareOffering, error) {
	rows, err := v.CareOfferingRows.List(ctx)
	if err != nil {
		return nil, err
	}
	return PublicCareOfferingList(rows), nil
}

// ListByPhase lists the catalog of one phase.
func (v *CareOfferingValues) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollment.CareOffering, error) {
	rows, err := v.CareOfferingRows.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	return PublicCareOfferingList(rows), nil
}

// GetByID loads one offering.
func (v *CareOfferingValues) GetByID(ctx context.Context, id int64) (*enrollment.CareOffering, error) {
	row, err := v.CareOfferingRows.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return PublicCareOffering(row), nil
}

// Create saves a new offering and returns the stored value.
func (v *CareOfferingValues) Create(ctx context.Context, offering *enrollment.CareOffering) (*enrollment.CareOffering, error) {
	row, err := CareOfferingRowOf(offering)
	if err != nil {
		return nil, err
	}
	stored, err := v.CareOfferingRows.Create(ctx, row)
	if err != nil {
		return nil, err
	}
	return PublicCareOffering(stored), nil
}

// Update saves the offering and refreshes it with the stored state.
func (v *CareOfferingValues) Update(ctx context.Context, offering *enrollment.CareOffering) error {
	row, err := CareOfferingRowOf(offering)
	if err != nil {
		return err
	}
	if err := v.CareOfferingRows.Update(ctx, row); err != nil {
		return err
	}
	*offering = *PublicCareOffering(row)
	return nil
}

// Clone copies an offering into a target phase.
func (v *CareOfferingValues) Clone(ctx context.Context, sourceID, targetPhaseID int64) (*enrollment.CareOffering, error) {
	row, err := v.CareOfferingRows.Clone(ctx, sourceID, targetPhaseID)
	if err != nil {
		return nil, err
	}
	return PublicCareOffering(row), nil
}
