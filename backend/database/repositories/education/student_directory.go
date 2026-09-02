package education

import (
	"context"
	"errors"
)

// DirectoryStudent is the People Directory projection the grade transition
// reads: users.students belongs to that owner (#2662), so every cohort read
// and every class or status write goes through StudentDirectory.
type DirectoryStudent struct {
	ID          int64
	PersonID    int64
	SchoolClass string
	Status      string
}

// StudentDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type StudentDirectory interface {
	// ListStudentsByID returns the tenant's rows for ids, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
	// ListStudentsByClasses returns the non-alumni students of the classes
	// ordered by class, then id.
	ListStudentsByClasses(ctx context.Context, classes []string) ([]DirectoryStudent, error)
	// ListSchoolClasses returns the distinct non-empty classes of non-alumni
	// students, ordered.
	ListSchoolClasses(ctx context.Context) ([]string, error)
	PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error)
	RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error)
	GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error)
	GraduateStudents(ctx context.Context, ids []int64) (int64, error)
	ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error)
}

var errStudentDirectoryRequired = errors.New("education repositories: student directory is not bound")
