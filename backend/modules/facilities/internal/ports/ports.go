package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
)

type Store interface {
	FindByID(context.Context, int64) (domain.Room, bool, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Room, domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
