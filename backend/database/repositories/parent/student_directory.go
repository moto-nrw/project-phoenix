package parent

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// DirectoryStudent is the People Directory projection the parent-portal
// repositories read. users.students belongs to that owner (#2662); the
// cross-tenant joins that used to read it are now guardian-link reads plus
// a directory lookup inside the same admin transaction.
type DirectoryStudent struct {
	ID            int64
	TenantID      int64
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

// StudentDirectory is bound by the composition root; it fails while
// unbound instead of falling back to a foreign query.
type StudentDirectory interface {
	// ListStudentsByID returns the rows for ids across every tenant the
	// admin transaction can see, alumni included.
	ListStudentsByID(ctx context.Context, ids []int64) ([]DirectoryStudent, error)
}

var errStudentDirectoryRequired = errors.New("parent repositories: student directory is not bound")

// parseDirectoryDate turns the directory's YYYY-MM-DD calendar day into the
// legacy model's timezone.Date; empty stays nil.
func parseDirectoryDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	date, err := timezone.ParseDate(value)
	if err != nil {
		return nil
	}
	return &date
}

func studentsByID(ctx context.Context, directory StudentDirectory, ids []int64) (map[int64]DirectoryStudent, error) {
	if directory == nil {
		return nil, errStudentDirectoryRequired
	}
	if len(ids) == 0 {
		return map[int64]DirectoryStudent{}, nil
	}
	students, err := directory.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]DirectoryStudent, len(students))
	for _, student := range students {
		result[student.ID] = student
	}
	return result, nil
}
