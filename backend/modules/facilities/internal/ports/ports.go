package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
)

type Store interface {
	Create(context.Context, domain.CreateRoom) (domain.Room, domain.OperationStats, error)
	Update(context.Context, domain.UpdateRoom) (domain.Room, domain.OperationStats, error)
	Delete(context.Context, int64) (domain.OperationStats, error)
	FindByID(context.Context, int64, string) (domain.Room, bool, domain.OperationStats, error)
	FindByName(context.Context, string) (domain.Room, bool, domain.OperationStats, error)
	FindToilet(context.Context, int64) (domain.Room, bool, domain.OperationStats, error)
	List(context.Context, domain.RoomFilter) ([]domain.Room, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Room, domain.OperationStats, error)
	LockByIDs(context.Context, []int64) ([]domain.Room, domain.OperationStats, error)
}

type Transaction interface {
	RunWrite(context.Context, func(context.Context) error) error
	AcquireLock(context.Context, string) error
}

type DeletionGuard func(context.Context, int64) error
type DeletionLock func(context.Context) error

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
