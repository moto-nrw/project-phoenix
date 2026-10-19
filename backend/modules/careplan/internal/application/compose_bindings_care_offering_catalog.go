package application

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type CareOfferings struct {
	careplan.CareOfferingCatalogCapability
	careplan.BookingMaterializationCapability
	careplan.OfferingChangeCapability
	careplan.PickupAdjustments
}
