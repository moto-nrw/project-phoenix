package users

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The staff student directory moved to its owner in #3349. What stays here is
// the contract the retained handlers call; the filter surface, the SQL and the
// ordering belong to modules/peopledirectory, and the composition root binds
// an implementation (database/repositories.NewStudentDirectory).

// StudentDirectoryReader is the filtered, paginated directory read.
type StudentDirectoryReader interface {
	// ListStudents returns one ordered page of the selection.
	ListStudents(ctx context.Context, filter userModels.StudentDirectoryFilter) ([]*userModels.Student, error)
	// CountStudents counts the same selection without its page window.
	CountStudents(ctx context.Context, filter userModels.StudentDirectoryFilter) (int, error)
	// ListStudentIDs returns the ids of every non-alumni child.
	ListStudentIDs(ctx context.Context) ([]int64, error)
}
