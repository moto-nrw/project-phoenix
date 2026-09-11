package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"
)

// Service answers structure reads inside the caller's ambient transaction:
// education.groups is tenant-scoped, so the caller's RLS context decides
// visibility and the service never opens an admin transaction of its own.
type Service struct {
	store   ports.Store
	observe ports.Observer
}

func New(store ports.Store, observe ports.Observer) *Service {
	if store == nil || observe == nil {
		panic("school structure application: all dependencies are required")
	}
	return &Service{store: store, observe: observe}
}

func (s *Service) FindByID(ctx context.Context, id int64) (result domain.Group, err error) {
	err = s.run("find_group", func(stats *domain.OperationStats) error {
		group, found, queryStats, findErr := s.store.FindByID(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		result = group
		return nil
	})
	return result, err
}

func (s *Service) ListByIDs(ctx context.Context, ids []int64) (result []domain.Group, err error) {
	err = s.run("list_groups_by_id", func(stats *domain.OperationStats) error {
		groups, queryStats, listErr := s.store.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		result = groups
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
