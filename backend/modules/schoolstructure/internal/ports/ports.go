package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

type Store interface {
	List(context.Context, int64, int) ([]domain.Group, domain.OperationStats, error)
	FindByID(context.Context, int64) (domain.Group, bool, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Group, domain.OperationStats, error)
	// CountStudentTransitionHistory counts the grade-transition ledger rows
	// of the tenant that still carry the child's name.
	CountStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (int, domain.OperationStats, error)
	// AnonymizeStudentTransitionHistory replaces the child's name and clears
	// the tag on the tenant's ledger rows; returns the rows changed.
	AnonymizeStudentTransitionHistory(ctx context.Context, tenantID, studentID int64) (int64, domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
