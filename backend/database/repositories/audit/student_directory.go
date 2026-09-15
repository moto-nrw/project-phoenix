package audit

import (
	"context"
	"errors"
)

// DirectoryStudent is the People Directory projection the audits read:
// users.students belongs to that owner (#2662), so the alumnus exclusion is
// resolved through it instead of a foreign join.
type DirectoryStudent struct {
	ID      int64
	Alumnus bool
}

// StudentDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type StudentDirectory interface {
	// ListStudentsByID returns the tenant's rows for ids, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
}

var errStudentDirectoryRequired = errors.New("audit repositories: student directory is not bound")
