package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func NewOffboardingExecute(service *Offboarding) func(ctx context.Context, staffID int64, from string, revision string) (timetable.OffboardingCounts, error) {
	return func(ctx context.Context, staffID int64, from, revision string) (timetable.OffboardingCounts, error) {
		result, err := service.Execute(ctx, staffID, from, revision)
		return OffboardingCountsToPublic(result), MapError(err)
	}
}
