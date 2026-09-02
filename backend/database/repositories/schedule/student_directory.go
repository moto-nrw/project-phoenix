package schedule

import (
	"context"
	"errors"
)

// DirectoryStudent is the People Directory projection the schedule
// repositories read: users.students belongs to that owner (#2662), so the
// alumnus exclusion is resolved through it instead of a foreign join.
type DirectoryStudent struct {
	ID      int64
	Alumnus bool
}

// StudentDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type StudentDirectory interface {
	// ListStudentsByID returns the tenant's rows for ids, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
	// LockStudent takes the student row FOR UPDATE inside the caller's
	// transaction and returns an error wrapping sql.ErrNoRows when the
	// tenant has no such student.
	LockStudent(ctx context.Context, studentID int64) error
}

var (
	errStudentDirectoryRequired = errors.New("schedule repositories: student directory is not bound")
	// ErrStudentNotFound is what StudentDirectory.LockStudent reports when
	// the tenant has no such student; the repository maps it to the
	// sql.ErrNoRows contract of the care-day writers.
	ErrStudentNotFound = errors.New("schedule repositories: student not found")
)
