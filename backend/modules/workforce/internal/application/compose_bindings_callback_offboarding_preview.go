package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewOffboardingPreview(service *Service) func(ctx context.Context, staffID int64, from string) (workforce.OffboardingPreview, error) {
	return func(ctx context.Context, staffID int64, from string) (workforce.OffboardingPreview, error) {
		result, err := service.PreviewStaffOffboarding(ctx, staffID, from)
		return workforce.OffboardingPreview{Counts: OffboardingCountsToPublic(result.Counts), Revision: result.Revision, Blocked: result.Blocked}, MapError(err)
	}
}
