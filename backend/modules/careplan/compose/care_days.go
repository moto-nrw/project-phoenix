package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
)

type CareParticipationResolver = application.CareParticipationResolver
type CareDayDependencies = application.CareDayDependencies

// NewCareDayQuery composes the care-day derivation over its required reads.
func NewCareDayQuery(deps CareDayDependencies) (careplan.CareDays, error) {
	return application.NewCareDayQuery(deps)
}
