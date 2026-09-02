package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

type Service struct {
	store   ports.Store
	tx      ports.Transaction
	observe ports.Observer
}

func New(store ports.Store, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("organization tenancy application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, observe: observe}
}

func (s *Service) Create(ctx context.Context, input domain.CreateOrganization) (result domain.Organization, err error) {
	err = s.run(ctx, "create_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, findErr := s.store.FindBySlug(txCtx, input.Slug)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if found {
			return domain.ErrSlugConflict
		}
		var createStats domain.OperationStats
		result, createStats, err = s.store.Create(txCtx, input)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) Update(ctx context.Context, input domain.UpdateOrganization) (result domain.Organization, err error) {
	err = s.run(ctx, "update_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, findErr := s.store.FindByID(txCtx, input.ID, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrNotFound
		}
		if existing.IsDeleted() {
			return domain.ErrAlreadyDeleted
		}
		if input.Slug != existing.Slug {
			taken, slugFound, slugStats, slugErr := s.store.FindBySlug(txCtx, input.Slug)
			stats.Add(slugStats)
			if slugErr != nil {
				return slugErr
			}
			if slugFound && taken.ID != input.ID {
				return domain.ErrSlugConflict
			}
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.Update(txCtx, input)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) SoftDelete(ctx context.Context, id int64) (result domain.Organization, err error) {
	err = s.run(ctx, "soft_delete_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, queryStats, err := s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrNotFound
		}
		if organization.IsDeleted() {
			return domain.ErrAlreadyDeleted
		}
		count, countStats, err := s.store.CountNonDeletedSchools(txCtx, id)
		stats.Add(countStats)
		if err != nil {
			return err
		}
		if count > 0 {
			return &domain.HasSchoolsError{Count: count}
		}
		deleteStats, err := s.store.SoftDelete(txCtx, id)
		stats.Add(deleteStats)
		if err != nil {
			return err
		}
		result, _, queryStats, err = s.store.FindByID(txCtx, id, "")
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) Restore(ctx context.Context, id int64) (result domain.Organization, err error) {
	err = s.run(ctx, "restore_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, queryStats, err := s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrNotFound
		}
		if !organization.IsDeleted() {
			return domain.ErrNotDeleted
		}
		restoreStats, err := s.store.Restore(txCtx, id)
		stats.Add(restoreStats)
		if err != nil {
			return err
		}
		result, _, queryStats, err = s.store.FindByID(txCtx, id, "")
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindByID(ctx context.Context, id int64) (result domain.Organization, err error) {
	err = s.run(ctx, "find_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, "")
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindForSchoolMutation(ctx context.Context, id int64) (result domain.Organization, err error) {
	err = s.run(ctx, "find_organization_for_school_mutation", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, "SHARE")
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindForMutation(ctx context.Context, id int64) (result domain.Organization, err error) {
	err = s.run(ctx, "find_organization_for_mutation", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindBySlug(ctx context.Context, slug string) (result domain.Organization, err error) {
	err = s.run(ctx, "find_organization_by_slug", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindBySlug(txCtx, slug)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) List(ctx context.Context) (result []domain.Organization, err error) {
	err = s.run(ctx, "list_organizations", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.List(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListByIDs(ctx context.Context, ids []int64) (result []domain.Organization, err error) {
	err = s.run(ctx, "list_organizations_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountByIDs(ctx context.Context, ids []int64) (result int, err error) {
	err = s.run(ctx, "count_organizations_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	return s.observeRun(ctx, operation, s.tx.RunAdmin, fn)
}

func (s *Service) runRead(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) (err error) {
	return s.observeRun(ctx, operation, s.tx.RunRead, fn)
}

func (s *Service) observeRun(ctx context.Context, operation string, run func(context.Context, func(context.Context) error) error, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = run(ctx, func(txCtx context.Context) error { return fn(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	return err
}
