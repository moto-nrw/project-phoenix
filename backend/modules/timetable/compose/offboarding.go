package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
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
	service := application.NewOffboarding(postgres.New(databaseRuntime(dependencies.DB)), transaction{}, dependencies.Sessions, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return timetable.NewOffboarding(
		func(ctx context.Context, staffID int64, from string) (timetable.OffboardingPreview, error) {
			result, err := service.Preview(ctx, staffID, from)
			return timetable.OffboardingPreview{Counts: offboardingCountsToPublic(result.Counts), Revision: result.Revision}, mapError(err)
		},
		func(ctx context.Context, staffID int64, from, revision string) (timetable.OffboardingCounts, error) {
			result, err := service.Execute(ctx, staffID, from, revision)
			return offboardingCountsToPublic(result), mapError(err)
		}), nil
}

func offboardingCountsToPublic(value domain.OffboardingCounts) timetable.OffboardingCounts {
	return timetable.OffboardingCounts{InstanceAssignments: value.InstanceAssignments, PlannedSupervisors: value.PlannedSupervisors}
}
