//go:build testhooks

package compose

import (
	"errors"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/uptrace/bun"
)

// NewBookingMaterialization composes the Care Plan booking materialization
// over the catalog it applies the timetable-link rules of.
func NewBookingMaterialization(deps BookingMaterializationDependencies) (careplan.BookingMaterializationCapability, error) {
	catalog, ok := deps.Catalog.(*application.CareOfferingCatalog)
	if !ok {
		return nil, errors.New("booking materialization: the care-offering catalog must be composed by NewCareOfferingCatalog")
	}
	return application.NewBookingMaterialization(bookingDependencies(deps, catalog))
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
