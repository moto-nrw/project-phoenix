package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/ports"
)

type employmentProfile interface {
	~struct {
		MembershipID          int64
		StaffNotes            string
		EmploymentType        *string
		WorkTimeModelID       *int64
		PersonnelNumber       *string
		RotationAnchorDate    string
		BirthdayDisplayOptOut bool
	}
}
type employmentPort[Profile employmentProfile] interface {
	StaffEmployments(context.Context, []int64) (map[int64]Profile, error)
	SaveStaffEmployment(context.Context, Profile) error
	ClearStaffWorkTimeModel(context.Context, int64) error
}

func NewStaffEmployment[Profile employmentProfile](source employmentPort[Profile]) ports.StaffEmployment {
	return employmentBinding[Profile]{port: source}
}

type employmentBinding[Profile employmentProfile] struct{ port employmentPort[Profile] }

func (e employmentBinding[Profile]) StaffEmployments(ctx context.Context, ids []int64) (map[int64]domain.StaffEmployment, error) {
	values, err := e.port.StaffEmployments(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]domain.StaffEmployment, len(values))
	for id, value := range values {
		result[id] = domain.StaffEmployment(value)
	}
	return result, nil
}

func (e employmentBinding[Profile]) SaveStaffEmployment(ctx context.Context, value domain.StaffEmployment) error {
	return e.port.SaveStaffEmployment(ctx, Profile(value))
}

func (e employmentBinding[Profile]) ClearStaffWorkTimeModel(ctx context.Context, membershipID int64) error {
	return e.port.ClearStaffWorkTimeModel(ctx, membershipID)
}
