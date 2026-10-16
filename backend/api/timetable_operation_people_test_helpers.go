//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// TimetableTestStaffAccountPeople resolves every account to the person
// PersonID and every person to the staff member StaffID, for timetable route
// suites that only need the acting account's staff identity; the batch reads
// find nobody. It satisfies the operations' person port, which names the
// retained models those suites may not import (#3966).
type TimetableTestStaffAccountPeople struct {
	PersonID int64
	StaffID  int64
}

func (p TimetableTestStaffAccountPeople) FindByAccountID(context.Context, int64) (*timetable.Person, error) {
	return &timetable.Person{ID: p.PersonID}, nil
}

func (p TimetableTestStaffAccountPeople) GetStaffByPersonID(context.Context, int64) (*timetable.StaffMember, error) {
	return &timetable.StaffMember{ID: p.StaffID}, nil
}

func (TimetableTestStaffAccountPeople) GetByIDs(context.Context, []int64) (map[int64]*timetable.Person, error) {
	return nil, nil
}

func (TimetableTestStaffAccountPeople) GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*timetable.StaffMember, error) {
	return nil, nil
}
