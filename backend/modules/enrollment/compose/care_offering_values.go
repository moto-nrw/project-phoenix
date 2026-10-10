package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
)

// CareOfferingValues serves Care Plan's catalog administration to the
// enrollment routes in Enrollment's public offering values (#2734). Every
// operation runs through the row translation of CareOfferingRows, so the
// stored columns and the marked refusals are those of the rows.
type CareOfferingValues struct {
	rows *CareOfferingRows
}

// NewCareOfferingValues binds the values to the catalog.
func NewCareOfferingValues(catalog CareOfferingCatalogAdministration) *CareOfferingValues {
	return &CareOfferingValues{rows: NewCareOfferingRows(catalog)}
}

// List lists the tenant's catalog.
func (v *CareOfferingValues) List(ctx context.Context) ([]*enrollment.CareOffering, error) {
	rows, err := v.rows.List(ctx)
	if err != nil {
		return nil, err
	}
	return application.PublicCareOfferingList(rows), nil
}

// ListByPhase lists the catalog of one phase.
func (v *CareOfferingValues) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollment.CareOffering, error) {
	rows, err := v.rows.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	return application.PublicCareOfferingList(rows), nil
}

// GetByID loads one offering.
func (v *CareOfferingValues) GetByID(ctx context.Context, id int64) (*enrollment.CareOffering, error) {
	row, err := v.rows.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return application.PublicCareOffering(row), nil
}

// Create saves a new offering and returns the stored value.
func (v *CareOfferingValues) Create(ctx context.Context, offering *enrollment.CareOffering) (*enrollment.CareOffering, error) {
	row, err := application.CareOfferingRowOf(offering)
	if err != nil {
		return nil, err
	}
	stored, err := v.rows.Create(ctx, row)
	if err != nil {
		return nil, err
	}
	return application.PublicCareOffering(stored), nil
}

// Update saves the offering and refreshes it with the stored state.
func (v *CareOfferingValues) Update(ctx context.Context, offering *enrollment.CareOffering) error {
	row, err := application.CareOfferingRowOf(offering)
	if err != nil {
		return err
	}
	if err := v.rows.Update(ctx, row); err != nil {
		return err
	}
	*offering = *application.PublicCareOffering(row)
	return nil
}

// Delete deletes an offering.
func (v *CareOfferingValues) Delete(ctx context.Context, id int64) error {
	return v.rows.Delete(ctx, id)
}

// Clone copies an offering into a target phase.
func (v *CareOfferingValues) Clone(ctx context.Context, sourceID, targetPhaseID int64) (*enrollment.CareOffering, error) {
	row, err := v.rows.Clone(ctx, sourceID, targetPhaseID)
	if err != nil {
		return nil, err
	}
	return application.PublicCareOffering(row), nil
}

// ListBookingStats reports how full each offering of the phase is.
func (v *CareOfferingValues) ListBookingStats(ctx context.Context, phaseID int64) ([]enrollment.CareOfferingBookingStat, error) {
	return v.rows.ListBookingStats(ctx, phaseID)
}
