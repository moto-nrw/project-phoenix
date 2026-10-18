package ports

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentDirectoryReader is the filtered, paginated directory read.
type StudentDirectoryReader interface {
	// ListStudents returns one ordered page of the selection.
	ListStudents(ctx context.Context, filter peopleValues.StudentDirectoryFilter) ([]*peopleValues.StudentState, error)
	// CountStudents counts the same selection without its page window.
	CountStudents(ctx context.Context, filter peopleValues.StudentDirectoryFilter) (int, error)
	// ListStudentIDs returns the ids of every non-alumni child.
	ListStudentIDs(ctx context.Context) ([]int64, error)
}

// StudentDirectoryLocker is the owner's write-side row access: the locked read
// a caller takes before it writes, and the per-tenant photo gate that read
// pairs with.
type StudentDirectoryLocker interface {
	// GetStudentsByID reads the owned rows of the given children.
	GetStudentsByID(ctx context.Context, ids []int64) ([]*peopleValues.StudentState, error)
	// LockStudent re-reads one owned row and holds its lock for the caller's
	// transaction.
	LockStudent(ctx context.Context, id int64) (*peopleValues.StudentState, error)
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
