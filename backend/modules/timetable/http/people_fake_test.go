package timetablehttp

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// errFakeNoPerson is what fakePeople answers for an account without a
// configured person.
var errFakeNoPerson = errors.New("fake people: account has no person")

// fakePeople serves the People port from func fields for database-free
// handler tests. An unset field answers that the directory knows nothing:
// no names, no person for an account, no staff member, no attending child.
type fakePeople struct {
	PersonNamesFn             func(ctx context.Context, personIDs []int64) (map[int64]string, error)
	AccountPersonIDFn         func(ctx context.Context, accountID int64) (int64, error)
	AccountPersonNameFn       func(ctx context.Context, accountID int64) (string, error)
	StaffPersonIDFn           func(ctx context.Context, staffID int64) (int64, error)
	PersonStaffIDFn           func(ctx context.Context, personID int64) (int64, error)
	StaffNamesFn              func(ctx context.Context, staffIDs []int64) (map[int64]string, error)
	AttendingStudentPersonsFn func(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]int64, error)
	StudentAttendsFn          func(ctx context.Context, studentID int64, day calendar.Date) (bool, error)
}

// staffAccountPeople resolves every account to the given person and that
// person to the given staff member.
func staffAccountPeople(personID, staffID int64) *fakePeople {
	return &fakePeople{
		AccountPersonIDFn: func(context.Context, int64) (int64, error) { return personID, nil },
		PersonStaffIDFn:   func(context.Context, int64) (int64, error) { return staffID, nil },
	}
}

func (f *fakePeople) PersonNames(ctx context.Context, personIDs []int64) (map[int64]string, error) {
	if f.PersonNamesFn == nil {
		return map[int64]string{}, nil
	}
	return f.PersonNamesFn(ctx, personIDs)
}

func (f *fakePeople) AccountPersonID(ctx context.Context, accountID int64) (int64, error) {
	if f.AccountPersonIDFn == nil {
		return 0, errFakeNoPerson
	}
	return f.AccountPersonIDFn(ctx, accountID)
}

func (f *fakePeople) AccountPersonName(ctx context.Context, accountID int64) (string, error) {
	if f.AccountPersonNameFn == nil {
		return "", errFakeNoPerson
	}
	return f.AccountPersonNameFn(ctx, accountID)
}

func (f *fakePeople) StaffPersonID(ctx context.Context, staffID int64) (int64, error) {
	if f.StaffPersonIDFn == nil {
		return 0, nil
	}
	return f.StaffPersonIDFn(ctx, staffID)
}

func (f *fakePeople) PersonStaffID(ctx context.Context, personID int64) (int64, error) {
	if f.PersonStaffIDFn == nil {
		return 0, nil
	}
	return f.PersonStaffIDFn(ctx, personID)
}

func (f *fakePeople) StaffNames(ctx context.Context, staffIDs []int64) (map[int64]string, error) {
	if f.StaffNamesFn == nil {
		return map[int64]string{}, nil
	}
	return f.StaffNamesFn(ctx, staffIDs)
}

func (f *fakePeople) AttendingStudentPersons(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]int64, error) {
	if f.AttendingStudentPersonsFn == nil {
		return map[int64]int64{}, nil
	}
	return f.AttendingStudentPersonsFn(ctx, studentIDs, day)
}

func (f *fakePeople) StudentAttends(ctx context.Context, studentID int64, day calendar.Date) (bool, error) {
	if f.StudentAttendsFn == nil {
		return false, nil
	}
	return f.StudentAttendsFn(ctx, studentID, day)
}
