package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

func NewOffboardingPreview(service *Offboarding) func(ctx context.Context, staffID int64, from string) (timetable.OffboardingPreview, error) {
	return func(ctx context.Context, staffID int64, from string) (timetable.OffboardingPreview, error) {
		result, err := service.Preview(ctx, staffID, from)
		return timetable.OffboardingPreview{Counts: OffboardingCountsToPublic(result.Counts), Revision: result.Revision}, MapError(err)
	}
}
