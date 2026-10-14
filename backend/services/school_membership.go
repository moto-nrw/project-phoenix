package services

import (
	"context"

	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/uptrace/bun"
)

// NewSchoolMembership composes the staff/teacher/guest owner behind the legacy
// composition seam for test graphs and CLI roots that do not record
// observations. Production roots compose the module themselves (api/base.go)
// so runtime evidence is kept.
func NewSchoolMembership(db *bun.DB) (schoolmembership.Capability, error) {
	employment, err := workforceCompose.NewStaffEmployment(db, nil)
	if err != nil {
		return nil, err
	}
	return schoolMembershipCompose.New(schoolMembershipCompose.Dependencies{
		DB:         db,
		Observe:    func(schoolMembershipCompose.Observation) {},
		Employment: MembershipStaffEmployment(employment),
		ChildQuota: organizationCompose.NewChildQuotaLimits(),
	})
}

// MembershipStaffEmployment binds School Membership's consumer-owned
// employment port to the Workforce owner, translating the errors School
// Membership callers classify into its own vocabulary.
func MembershipStaffEmployment(employment workforce.StaffEmployments) schoolMembershipCompose.StaffEmployment {
	return membershipStaffEmployment{employment: employment}
}

type membershipStaffEmployment struct{ employment workforce.StaffEmployments }

func (e membershipStaffEmployment) StaffEmployments(ctx context.Context, ids []int64) (map[int64]schoolMembershipCompose.StaffEmploymentProfile, error) {
	values, err := e.employment.StaffEmployments(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]schoolMembershipCompose.StaffEmploymentProfile, len(values))
	for id, value := range values {
		result[id] = schoolMembershipCompose.StaffEmploymentProfile(value)
	}
	return result, nil
}

func (e membershipStaffEmployment) SaveStaffEmployment(ctx context.Context, value schoolMembershipCompose.StaffEmploymentProfile) error {
	return membershipEmploymentError(e.employment.SaveStaffEmployment(ctx, workforce.StaffEmployment(value)))
}

func (e membershipStaffEmployment) ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error {
	return membershipEmploymentError(e.employment.ClearStaffWorkTimeModel(ctx, membershipID))
}
