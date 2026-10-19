//go:build testhooks

package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentRepo "github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
	"github.com/uptrace/bun"
)

// NewBookingMaterialization composes the Care Plan booking materialization
// over the catalog it applies the timetable-link rules of.
func NewBookingMaterialization(deps BookingMaterializationDependencies) (careplan.BookingMaterializationCapability, error) {
	catalog, ok := deps.Catalog.(*application.CareOfferingCatalog)
	if !ok {
		return nil, errors.New("booking materialization: the care-offering catalog must be composed by NewCareOfferingCatalog")
	}
	records, err := application.NewBookingMaterialization(bookingDependencies(deps, catalog))
	if err != nil {
		return nil, err
	}
	return application.NewBookingCommands(records), nil
}

// NewCareOfferingCatalog composes the Care Plan care-offering catalog.
func NewCareOfferingCatalog(deps CareOfferingCatalogDependencies) (careplan.CareOfferingCatalogCapability, error) {
	return application.NewCareOfferingCatalog(catalogDependencies(deps))
}

// NewParentEnrollmentRequestRepository is the account's applications over the
// ambient tenant transaction, without the guardian links bound.
func NewParentEnrollmentRequestRepository(db *bun.DB, applications parentRepo.EnrollmentCommands, accounts parentRepo.AccountDirectory) parentModels.EnrollmentRequestRepository {
	return parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(TenantAmbientDatabase(db)), applications, accounts, nil)
}
