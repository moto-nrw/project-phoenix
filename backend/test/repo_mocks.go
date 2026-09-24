package test

// Test doubles for repository-shaped collaborators, following the
// configtest.Mock convention: exported fields configure the answers.

import (
	"context"
)

type FeedbackEntryCounterMock struct {
	Count int
	Err   error
}

func (m *FeedbackEntryCounterMock) CountForStudent(context.Context, int64) (int, error) {
	return m.Count, m.Err
}
