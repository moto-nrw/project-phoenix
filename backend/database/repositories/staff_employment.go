package repositories

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/uptrace/bun"
)

// NewStaffEmployment composes Workforce's owner of the staff employment
// profile (users.staff_employment_profiles, #2753).
func NewStaffEmployment(db *bun.DB) (workforce.StaffEmployments, error) {
	return workforceCompose.NewStaffEmployment(db)
}

// MustNewStaffEmployment is NewStaffEmployment for composition roots and test
// graphs that treat a composition failure as a programming error.
func MustNewStaffEmployment(db *bun.DB) workforce.StaffEmployments {
	employment, err := NewStaffEmployment(db)
	if err != nil {
		panic(err)
	}
	return employment
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

func membershipEmploymentError(err error) error {
	switch {
	case errors.Is(err, workforce.ErrPersonnelNumberTaken):
		return schoolmembership.ErrPersonnelNumberConflict
	case errors.Is(err, workforce.ErrStaffEmploymentNotFound):
		return schoolmembership.ErrStaffNotFound
	}
	return err
}

// WorkforceLiveStaffIDs answers Workforce's one question to School
// Membership: which of the memberships bound to a work-time template are still
// live. Workforce never reads users.staff_school_memberships itself.
func WorkforceLiveStaffIDs(membership schoolmembership.Capability) func(context.Context, []int64) ([]int64, error) {
	return func(ctx context.Context, ids []int64) ([]int64, error) {
		members, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids})
		if err != nil {
			return nil, err
		}
		live := make([]int64, 0, len(members))
		for _, member := range members {
			live = append(live, member.ID)
		}
		return live, nil
	}
}
