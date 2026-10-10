package test

// Test doubles for repository-shaped collaborators, following the
// configtest.Mock convention: exported fields configure the answers.

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
)

type FeedbackEntryCounterMock struct {
	Count int
	Err   error
}

func (m *FeedbackEntryCounterMock) CountForStudent(context.Context, int64) (int, error) {
	return m.Count, m.Err
}

// StaffAccountPeople resolves every account to the person PersonID and every
// person to the staff member StaffID, for callers that only need the acting
// account's staff identity; the batch reads find nobody. It took over the
// StaffAccount double of the retired services/users/userstest package (#3753).
type StaffAccountPeople struct {
	PersonID int64
	StaffID  int64
}

func (p StaffAccountPeople) FindByAccountID(context.Context, int64) (*users.Person, error) {
	person := &users.Person{}
	person.ID = p.PersonID
	return person, nil
}

func (p StaffAccountPeople) GetStaffByPersonID(context.Context, int64) (*users.Staff, error) {
	staff := &users.Staff{}
	staff.ID = p.StaffID
	return staff, nil
}

func (StaffAccountPeople) GetByIDs(context.Context, []int64) (map[int64]*users.Person, error) {
	return nil, nil
}

func (StaffAccountPeople) GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*users.Staff, error) {
	return nil, nil
}
