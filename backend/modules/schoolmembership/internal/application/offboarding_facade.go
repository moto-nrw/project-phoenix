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
		},
		schoolmembership.Reinstatement{
			Staff: func(ctx context.Context, staffID int64) (schoolmembership.Staff, error) {
				value, err := service.ReinstateStaff(ctx, staffID)
				return staffToPublic(value), MapError(err)
			},
			Teacher: func(ctx context.Context, teacherID int64) (schoolmembership.Teacher, error) {
				value, err := service.ReinstateTeacher(ctx, teacherID)
				return teacherToPublic(value), MapError(err)
			},
		})
}
