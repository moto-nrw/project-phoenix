package services

import (
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
	supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

// NewSchulhofProjection binds the retained workflow to its public read projection.
func NewSchulhofProjection(source facilities.SchulhofService) supervisiondashboard.Yard {
	return supervisiondashboardlegacy.NewYard(source)
}
