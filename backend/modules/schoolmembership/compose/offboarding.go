package compose

import (
	"context"
	"time"

	application "github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/application"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// NewOffboarding binds the Membership-owned part of staff offboarding and of
// the automatic deletion of retired staff (#3963). It uses the same store and
// ambient transaction as the regular Membership facade.
func NewOffboarding(dependencies Dependencies) (*schoolmembership.Offboarding, error) {
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return schoolmembership.NewOffboarding(
		func(ctx context.Context, staffID int64) (schoolmembership.RetirementPreview, error) {
			result, err := service.PreviewRetirement(ctx, staffID)
			return schoolmembership.RetirementPreview{Retirement: schoolmembership.Retirement(result.Retirement), Revision: result.Revision}, application.MapError(err)
		},
		func(ctx context.Context, staffID int64, revision string) (schoolmembership.Retirement, error) {
			result, err := service.RetireStaff(ctx, staffID, revision)
			return schoolmembership.Retirement(result), application.MapError(err)
		},
		func(ctx context.Context, retiredBefore time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, error) {
			result, found, err := service.EraseNextRetiredStaff(ctx, retiredBefore, afterID)
			return schoolmembership.RetiredStaffErasure(result), found, application.MapError(err)
		}), nil
}
