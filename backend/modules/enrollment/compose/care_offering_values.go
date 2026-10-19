package compose

import applicationBindings "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"

// NewCareOfferingValues binds the values to the catalog.
func NewCareOfferingValues(catalog CareOfferingCatalogAdministration) *CareOfferingValues {
	return &CareOfferingValues{CareOfferingValues: applicationBindings.CareOfferingValues{Rows: NewCareOfferingRows(catalog)}}
}

type CareOfferingValues struct {
	applicationBindings.CareOfferingValues
}
