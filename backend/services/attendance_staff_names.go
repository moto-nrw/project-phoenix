package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

type attendanceNameStaff interface {
	FindByID(context.Context, any) (*users.Staff, error)
}

type attendanceNamePeople interface {
	Get(context.Context, interface{}) (*users.Person, error)
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
	return fmt.Sprintf("%s %s", person.FirstName, person.LastName), nil
}
