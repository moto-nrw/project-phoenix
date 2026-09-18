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

// StudentDirectoryLocker is the owner's write-side row access: the locked read
// a caller takes before it writes, and the per-tenant photo gate that read
// pairs with.
type StudentDirectoryLocker interface {
	// GetStudentsByID reads the owned rows of the given children.
	GetStudentsByID(ctx context.Context, ids []int64) ([]*userModels.Student, error)
	// LockStudent re-reads one owned row and holds its lock for the caller's
	// transaction.
	LockStudent(ctx context.Context, id int64) (*userModels.Student, error)
	// LockPhotoFeature takes the per-tenant photo gate. The caller takes it
	// before LockStudent, which is the order the owner's own photo writes use.
	LockPhotoFeature(ctx context.Context) error
	// LockClassWritesShared takes the shared per-tenant class-writes gate.
	LockClassWritesShared(ctx context.Context) error
}

// StudentDirectoryAccess is the whole owner surface this service reaches the
// child table through.
type StudentDirectoryAccess interface {
	StudentDirectoryReader
	StudentDirectoryLocker
}
