package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func NewOffboardingFacade(service *Service) *schoolmembership.Offboarding {
	return schoolmembership.NewOffboarding(
		func(ctx context.Context, staffID int64) (schoolmembership.RetirementPreview, error) {
			result, err := service.PreviewRetirement(ctx, staffID)
			return schoolmembership.RetirementPreview{Retirement: schoolmembership.Retirement(result.Retirement), Revision: result.Revision}, MapError(err)
		},
		func(ctx context.Context, staffID int64, revision string) (schoolmembership.Retirement, error) {
			result, err := service.RetireStaff(ctx, staffID, revision)
			return schoolmembership.Retirement(result), MapError(err)
		},
		func(ctx context.Context, retiredBefore time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, error) {
			result, found, err := service.EraseNextRetiredStaff(ctx, retiredBefore, afterID)
			return schoolmembership.RetiredStaffErasure(result), found, MapError(err)
		})
}
