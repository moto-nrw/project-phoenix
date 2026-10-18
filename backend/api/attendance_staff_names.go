package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

type attendanceNameStaff interface {
	FindByID(context.Context, any) (*peopleValues.Staff, error)
}

type attendanceNamePeople interface {
	Get(context.Context, interface{}) (*peopleValues.DirectoryPersonSnapshot, error)
}

type attendanceStaffNames struct {
	staff  attendanceNameStaff
	people attendanceNamePeople
}

// NewAttendanceStaffNames retains the staff and person lookups used by attendance.
func NewAttendanceStaffNames(staff attendanceNameStaff, people attendanceNamePeople) presenceservice.AttendanceStaffNames {
	return attendanceStaffNames{staff: staff, people: people}
}

func (q attendanceStaffNames) StaffName(ctx context.Context, id int64) (string, error) {
	staff, err := q.staff.FindByID(ctx, id)
	if err != nil || staff == nil {
		return "", err
	}
	person, err := q.people.Get(ctx, staff.PersonID)
	if err != nil || person == nil {
		return "", err
	}
	return person.GetFullName(), nil
}
