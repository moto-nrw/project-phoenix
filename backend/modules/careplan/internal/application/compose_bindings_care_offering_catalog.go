package application

import (
	"github.com/moto-ogs/project-phoenix/modules/careplan"
)

type CareOfferings struct {
	careplan.CareOfferingCatalogCapability
	careplan.BookingMaterializationCapability
	careplan.OfferingChangeCapability
	careplan.PickupAdjustments
}
