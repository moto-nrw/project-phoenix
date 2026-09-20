package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
)

type CareDayRecords = application.CareDayRecords
type CareParticipationResolver = application.CareParticipationResolver
type CareDayDependencies = application.CareDayDependencies

func NewCareDays(deps CareDayDependencies) careplan.CareDayQuery {
	return application.NewCareDays(deps)
}

func WireCareParticipation(service careplan.CareDayQuery, resolver CareParticipationResolver) {
	application.WireCareParticipation(service, resolver)
}
