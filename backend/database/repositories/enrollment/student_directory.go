package enrollment

import (
	"context"
	"errors"
)

// DirectoryStudent is the People Directory projection the enrollment
// repositories read: users.students belongs to that owner (#2662), so the
// class and the alumnus exclusion are resolved through it instead of a
// foreign join.
type DirectoryStudent struct {
	ID          int64
	SchoolClass string
	Status      string
	Alumnus     bool
	// EnrolledFrom and EnrolledUntil are calendar days in YYYY-MM-DD, empty
	// when unset; the phase-expiry report binds them as date arrays.
	EnrolledFrom  string
	EnrolledUntil string
}

// StudentDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type StudentDirectory interface {
	// ListStudentsByID returns the tenant's rows for ids, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
	// ListEnrolledStudents returns every non-alumni student of the current
	// tenant.
	ListEnrolledStudents(ctx context.Context) ([]DirectoryStudent, error)
}

var errStudentDirectoryRequired = errors.New("enrollment repositories: student directory is not bound")
