package services

import (
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
)

// NewSchulhofProjection binds the retained workflow to its public read projection.
func NewSchulhofProjection(source facilitiesCompose.SchulhofService) supervisiondashboard.Yard {
	return supervisiondashboardlegacy.NewYard(source)
}
