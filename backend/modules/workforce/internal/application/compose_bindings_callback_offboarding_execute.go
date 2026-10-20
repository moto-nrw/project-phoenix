package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewOffboardingExecute(service *Service, appendDeletion func(context.Context, workforce.StaffAbsence, int64) error) func(ctx context.Context, staffID int64, actorID int64, from string, revision string) (workforce.OffboardingCounts, error) {
	return func(ctx context.Context, staffID, actorID int64, from, revision string) (workforce.OffboardingCounts, error) {
		return service.ExecuteStaffOffboarding(ctx, staffID, actorID, from, revision, appendDeletion)
	}
}
