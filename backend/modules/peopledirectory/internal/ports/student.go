package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentStore is the persistence port over users.students. Reads honour the
// tenant in context when one is present; inside an admin transaction they
// span every tenant. Writes always require a tenant.
type StudentStore interface {
	// ListByIDs returns the rows for ids, alumni included.
	ListByIDs(context.Context, []int64) ([]domain.Student, domain.OperationStats, error)
	ListNamesByIDs(context.Context, []int64) ([]domain.StudentName, domain.OperationStats, error)
	// ListByClasses returns the non-alumni rows of the classes, ordered by
	// class then id.
	ListByClasses(context.Context, []string) ([]domain.Student, domain.OperationStats, error)
	// ListEnrolled returns every non-alumni row of the current tenant.
	ListEnrolled(context.Context) ([]domain.Student, domain.OperationStats, error)
	// ListClasses returns the distinct non-empty classes of non-alumni rows.
	ListClasses(context.Context) ([]string, domain.OperationStats, error)
	ListByStatusFlag(context.Context, string) ([]domain.Student, domain.OperationStats, error)
	// Lock takes the row FOR UPDATE and reports whether it exists.
	Lock(context.Context, int64) (bool, domain.OperationStats, error)
	Promote(ctx context.Context, ids []int64, fromClass, toClass string) (int64, domain.OperationStats, error)
	RevertClass(ctx context.Context, id int64, fromClass, toClass string) (int64, domain.OperationStats, error)
	GraduateByClasses(context.Context, []string) (int64, domain.OperationStats, error)
	GraduateByIDs(context.Context, []int64) (int64, domain.OperationStats, error)
	// Reactivate moves alumni back to status and returns the ids it changed.
	Reactivate(ctx context.Context, ids []int64, status string) ([]int64, domain.OperationStats, error)
	ClearStatusFlags(ctx context.Context, ids []int64, status string) (int64, domain.OperationStats, error)
}
