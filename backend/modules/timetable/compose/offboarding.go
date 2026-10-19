package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

type OffboardingDependencies struct {
	DB       *bun.DB
	Sessions timetable.SessionFacts
	Observe  func(Observation)
}

func NewOffboarding(dependencies OffboardingDependencies) (*timetable.Offboarding, error) {
	if dependencies.DB == nil || dependencies.Sessions == nil || dependencies.Observe == nil {
		return nil, errors.New("timetable: offboarding database, session facts and observer are required")
	}
	service := application.NewOffboarding(postgres.New(postgres.DatabaseRuntime(dependencies.DB)), transaction{}, dependencies.Sessions, application.NewOffboardingObserver(dependencies.Observe))
	return timetable.NewOffboarding(
		application.NewOffboardingPreview(service),
		application.NewOffboardingExecute(service)), nil
}
