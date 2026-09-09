package devicescan

import (
	"context"
	"errors"
)

var ErrFeedbackStudentNotFound = errors.New("student not found")

type FeedbackStudent struct {
	ID      int64
	Alumnus bool
}

// FeedbackStudents holds the student row lock until the caller's tenant
// transaction completes. A graduation cannot race the subsequent feedback
// write. No unlocked lookup is available through this interface.
type FeedbackStudents interface {
	LockFeedbackStudent(context.Context, int64) (FeedbackStudent, error)
}
