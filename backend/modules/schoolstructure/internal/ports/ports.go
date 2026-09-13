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
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
