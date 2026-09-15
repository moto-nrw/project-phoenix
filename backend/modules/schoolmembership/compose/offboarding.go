package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// NewOffboarding binds the Membership-owned part of staff offboarding. It uses
// the same store and ambient transaction as the regular Membership facade.
func NewOffboarding(dependencies Dependencies) (*schoolmembership.Offboarding, error) {
	service, err := newApplication(dependencies)
	if err != nil {
		return nil, err
	}
	return schoolmembership.NewOffboarding(
		func(ctx context.Context, staffID int64) (schoolmembership.RetirementPreview, error) {
			result, err := service.PreviewRetirement(ctx, staffID)
			return schoolmembership.RetirementPreview{Retirement: retirementToPublic(result.Retirement), Revision: result.Revision}, mapError(err)
		},
		func(ctx context.Context, staffID int64, revision string) (schoolmembership.Retirement, error) {
			result, err := service.RetireStaff(ctx, staffID, revision)
			return retirementToPublic(result), mapError(err)
		}), nil
}

func retirementToPublic(result domain.Retirement) schoolmembership.Retirement {
	return schoolmembership.Retirement{StaffID: result.StaffID, PersonID: result.PersonID, TeacherID: result.TeacherID, GroupAssignments: result.GroupAssignments, ClassAssignments: result.ClassAssignments}
}
