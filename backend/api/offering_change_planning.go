package api

import (
	owners "github.com/moto-ogs/project-phoenix/roots/owners"

	carePlanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
)

var _ carePlanCompose.OfferingChangePlanning = owners.ManualPlanningReader{}
