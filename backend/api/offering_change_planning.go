package api

import (
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

var _ carePlanCompose.OfferingChangePlanning = owners.ManualPlanningReader{}
