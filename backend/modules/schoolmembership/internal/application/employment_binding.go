package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
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

// employmentOf is the Workforce half of the staff fields: what StaffFields
// carries besides the person, stored by Workforce under the membership ID.
func employmentOf(fields schoolmembership.StaffFields, membershipID int64) domain.StaffEmployment {
	return domain.StaffEmployment{
		MembershipID: membershipID, StaffNotes: fields.StaffNotes, EmploymentType: fields.EmploymentType,
		WorkTimeModelID: fields.WorkTimeModelID, PersonnelNumber: fields.PersonnelNumber,
		RotationAnchorDate: fields.RotationAnchorDate, BirthdayDisplayOptOut: fields.BirthdayDisplayOptOut,
	}
}

// staffEmployment is the employment half a staff member carries.
func staffEmployment(staff schoolmembership.Staff) domain.StaffEmployment {
	return domain.StaffEmployment{
		MembershipID: staff.ID, StaffNotes: staff.StaffNotes, EmploymentType: staff.EmploymentType,
		WorkTimeModelID: staff.WorkTimeModelID, PersonnelNumber: staff.PersonnelNumber,
		RotationAnchorDate: staff.RotationAnchorDate, BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut,
	}
}

// staffWithEmployment returns the staff member with the employment fields set.
func staffWithEmployment(staff schoolmembership.Staff, employment domain.StaffEmployment) schoolmembership.Staff {
	staff.StaffNotes, staff.EmploymentType = employment.StaffNotes, employment.EmploymentType
	staff.WorkTimeModelID, staff.PersonnelNumber = employment.WorkTimeModelID, employment.PersonnelNumber
	staff.RotationAnchorDate, staff.BirthdayDisplayOptOut = employment.RotationAnchorDate, employment.BirthdayDisplayOptOut
	return staff
}
