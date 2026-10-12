package enrollmenthttp

import (
	"context"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// CareOfferingCatalog is the Care Plan care-offering catalog as these routes
// use it (#3559), with the offerings decoded for rendering.
type CareOfferingCatalog interface {
	List(ctx context.Context) ([]*CareOffering, error)
	ListByPhase(ctx context.Context, phaseID int64) ([]*CareOffering, error)
	GetByID(ctx context.Context, id int64) (*CareOffering, error)
	Create(ctx context.Context, offering *CareOffering) (*CareOffering, error)
	Update(ctx context.Context, offering *CareOffering) error
	Delete(ctx context.Context, id int64) error
	Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*CareOffering, error)
	ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error)
}

// CareOfferingCatalogOwner is the catalog administration in Enrollment's
// public offering values. The composition root binds it to Care Plan through
// Enrollment's translation, which also marks Care Plan's refusals with
// Enrollment's public values.
type CareOfferingCatalogOwner interface {
	List(ctx context.Context) ([]*capability.CareOffering, error)
	ListByPhase(ctx context.Context, phaseID int64) ([]*capability.CareOffering, error)
	GetByID(ctx context.Context, id int64) (*capability.CareOffering, error)
	Create(ctx context.Context, offering *capability.CareOffering) (*capability.CareOffering, error)
	Update(ctx context.Context, offering *capability.CareOffering) error
	Delete(ctx context.Context, id int64) error
	Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*capability.CareOffering, error)
	ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error)
}

// NewCareOfferingCatalog decodes the owner's catalog for these routes. A nil
// owner yields nil.
func NewCareOfferingCatalog(owner CareOfferingCatalogOwner) CareOfferingCatalog {
	if owner == nil {
		return nil
	}
	return careOfferingCatalog{owner: owner}
}

type careOfferingCatalog struct{ owner CareOfferingCatalogOwner }

func (c careOfferingCatalog) List(ctx context.Context) ([]*CareOffering, error) {
	values, err := c.owner.List(ctx)
	if err != nil {
		return nil, err
	}
	return careOfferingValues(values)
}

func (c careOfferingCatalog) ListByPhase(ctx context.Context, phaseID int64) ([]*CareOffering, error) {
	values, err := c.owner.ListByPhase(ctx, phaseID)
	if err != nil {
		return nil, err
	}
	return careOfferingValues(values)
}

func (c careOfferingCatalog) GetByID(ctx context.Context, id int64) (*CareOffering, error) {
	return decodedCareOffering(c.owner.GetByID(ctx, id))
}

func (c careOfferingCatalog) Create(ctx context.Context, offering *CareOffering) (*CareOffering, error) {
	value, err := careOfferingInput(offering)
	if err != nil {
		return nil, err
	}
	return decodedCareOffering(c.owner.Create(ctx, value))
}

// Update saves the offering and refreshes it with the stored state.
func (c careOfferingCatalog) Update(ctx context.Context, offering *CareOffering) error {
	value, err := careOfferingInput(offering)
	if err != nil {
		return err
	}
	if err := c.owner.Update(ctx, value); err != nil {
		return err
	}
	stored, err := careOfferingValue(value)
	if err != nil {
		return err
	}
	*offering = *stored
	return nil
}

func (c careOfferingCatalog) Delete(ctx context.Context, id int64) error {
	return c.owner.Delete(ctx, id)
}

func (c careOfferingCatalog) Clone(ctx context.Context, sourceID int64, targetPhaseID int64) (*CareOffering, error) {
	return decodedCareOffering(c.owner.Clone(ctx, sourceID, targetPhaseID))
}

func (c careOfferingCatalog) ListBookingStats(ctx context.Context, phaseID int64) ([]CareOfferingBookingStat, error) {
	return c.owner.ListBookingStats(ctx, phaseID)
}

func decodedCareOffering(value *capability.CareOffering, err error) (*CareOffering, error) {
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return careOfferingValue(value)
}
