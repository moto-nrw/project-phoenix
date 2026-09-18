package test

// Func-field mocks for users.StaffRepository,
// following the configtest.Mock convention: exported XxxFn fields, each
// method delegates to its Fn field, and a nil field returns zero values.

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

// StaffRepoMock is a func-field test double for the staff lookups a test
// drives; it embeds the repository contract, so a method nothing configured
// panics instead of pretending to answer. Add a field when a test needs one.
type StaffRepoMock struct {
	users.StaffRepository
	FindByPersonIDFn func(ctx context.Context, personID int64) (*users.Staff, error)
}

func (m *StaffRepoMock) FindByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	if m.FindByPersonIDFn != nil {
		return m.FindByPersonIDFn(ctx, personID)
	}
	return nil, nil
}
