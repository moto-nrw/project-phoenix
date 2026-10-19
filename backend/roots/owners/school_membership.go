package owners

import (
	"context"
	"errors"

	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// NewSchoolMembership composes the School Membership owner for test graphs
// and CLI roots that record no observations. The serving root composes the
// observed owner (composeSchoolMembership) so runtime evidence is kept.
func NewSchoolMembership(db transactionruntime.Database) (schoolmembership.Capability, error) {
	employment, err := workforceCompose.NewStaffEmployment(db.Pool(), nil)
	if err != nil {
		return nil, err
	}
	return schoolMembershipCompose.New(schoolMembershipCompose.Dependencies{
		DB:         db.Pool(),
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
	return MembershipEmploymentError(e.employment.SaveStaffEmployment(ctx, workforce.StaffEmployment(value)))
}

// MembershipEmploymentError restates Workforce's employment refusals in the
// School Membership vocabulary the staff adapters map.
func MembershipEmploymentError(err error) error {
	switch {
	case errors.Is(err, workforce.ErrPersonnelNumberTaken):
		return schoolmembership.ErrPersonnelNumberConflict
	case errors.Is(err, workforce.ErrStaffEmploymentNotFound):
		return schoolmembership.ErrStaffNotFound
	}
	return err
}
