package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Service is the School Structure capability. It answers structure reads
// inside the caller's ambient transaction: education.groups is
// tenant-scoped, so the caller's RLS context decides visibility and the
// service never opens an admin transaction of its own. The retention of
// grade transitions (#4045) comes with it for the nightly cleanup.
type Service struct {
	*TransitionRetention
	store   ports.Store
	observe ports.Observer
}

var _ schoolstructure.Owner = (*Service)(nil)

func New(store ports.Store, retention *TransitionRetention, observe ports.Observer) *Service {
	if store == nil || retention == nil || observe == nil {
		panic("school structure application: all dependencies are required")
	}
	return &Service{TransitionRetention: retention, store: store, observe: observe}
}

func (s *Service) FindGroup(ctx context.Context, id int64) (schoolstructure.Group, error) {
	if id <= 0 {
		return schoolstructure.Group{}, schoolstructure.ErrInvalidGroup
	}
	var result domain.Group
	err := s.run("find_group", func(stats *domain.OperationStats) error {
		group, found, queryStats, findErr := s.store.FindByID(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return schoolstructure.ErrGroupNotFound
		}
		result = group
		return nil
	})
	return toPublic(result), err
}

// ListGroups returns at most limit groups of the explicit tenant in context.
func (s *Service) ListGroups(ctx context.Context, limit int) ([]schoolstructure.Group, error) {
	if limit <= 0 || limit > 1000 {
		return nil, schoolstructure.ErrInvalidGroup
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	var result []domain.Group
	err = s.run("list_groups", func(stats *domain.OperationStats) error {
		groups, queryStats, listErr := s.store.List(ctx, tenantID.Int64(), limit)
		stats.Add(queryStats)
		result = groups
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return toPublicGroups(result), nil
}

// ListGroupsByID returns the groups visible in the caller's transaction for
// the given IDs, sorted by name. Missing IDs are simply absent: consumers
// rendering a display name treat absence like the former LEFT JOIN.
func (s *Service) ListGroupsByID(ctx context.Context, ids []int64) ([]schoolstructure.Group, error) {
	if len(ids) == 0 {
		return []schoolstructure.Group{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, schoolstructure.ErrInvalidGroup
		}
	}
	var result []domain.Group
	err := s.run("list_groups_by_id", func(stats *domain.OperationStats) error {
		groups, queryStats, listErr := s.store.ListByIDs(ctx, ids)
		stats.Add(queryStats)
		result = groups
		return listErr
	})
	if err != nil {
		return nil, err
	}
	return toPublicGroups(result), nil
}

func toPublic(value domain.Group) schoolstructure.Group {
	return schoolstructure.Group{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, RoomID: value.RoomID,
	}
}

func toPublicGroups(values []domain.Group) []schoolstructure.Group {
	result := make([]schoolstructure.Group, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result
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
