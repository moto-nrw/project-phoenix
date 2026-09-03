package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
)

// Service answers room reads inside the caller's ambient transaction:
// facilities.rooms is tenant-scoped, so the caller's RLS context decides
// visibility and the service never opens an admin transaction of its own.
type Service struct {
	store   ports.Store
	observe ports.Observer
}

func New(store ports.Store, observe ports.Observer) *Service {
	if store == nil || observe == nil {
		panic("facilities application: all dependencies are required")
	}
	return &Service{store: store, observe: observe}
}

func (s *Service) FindByID(ctx context.Context, id int64) (result domain.Room, err error) {
	err = s.run("find_room", func(stats *domain.OperationStats) error {
		room, found, queryStats, findErr := s.store.FindByID(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		result = room
		return nil
	})
	return result, err
}

func (s *Service) ListByIDs(ctx context.Context, ids []int64) (result []domain.Room, err error) {
	err = s.run("list_rooms_by_id", func(stats *domain.OperationStats) error {
		rooms, queryStats, listErr := s.store.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		result = rooms
		return listErr
	})
	return result, err
}

func (s *Service) run(operation string, fn func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(&stats)
	if err != nil {
		stats.Rows = 0
	}
	return err
}
