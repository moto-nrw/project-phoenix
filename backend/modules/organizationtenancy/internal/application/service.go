package application

import (
	"context"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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

func (s *Service) CreateOrganization(ctx context.Context, input organizationtenancy.CreateOrganization) (result organizationtenancy.Organization, err error) {
	input.Name, input.Slug = normalizeNameAndSlug(input.Name, input.Slug)
	if err := validateNameAndSlug(input.Name, input.Slug); err != nil {
		return organizationtenancy.Organization{}, err
	}
	err = s.run(ctx, "create_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, findErr := s.store.FindBySlug(txCtx, input.Slug)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if found {
			return organizationtenancy.ErrOrganizationSlugConflict
		}
		var createStats domain.OperationStats
		result, createStats, err = s.store.Create(txCtx, input)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateOrganization(ctx context.Context, input organizationtenancy.UpdateOrganization) (result organizationtenancy.Organization, err error) {
	input.Name, input.Slug = normalizeNameAndSlug(input.Name, input.Slug)
	if input.ID <= 0 {
		return organizationtenancy.Organization{}, organizationtenancy.ErrInvalidOrganization
	}
	if err := validateNameAndSlug(input.Name, input.Slug); err != nil {
		return organizationtenancy.Organization{}, err
	}
	err = s.run(ctx, "update_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, findErr := s.store.FindByID(txCtx, input.ID, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		if existing.IsDeleted() {
			return organizationtenancy.ErrOrganizationAlreadyDeleted
		}
		if err := s.requireOrganizationSlugChange(txCtx, existing, input.Slug, stats); err != nil {
			return err
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.Update(txCtx, input)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

// requireOrganizationSlugChange checks uniqueness only when the slug changes.
func (s *Service) requireOrganizationSlugChange(ctx context.Context, existing organizationtenancy.Organization, slug string, stats *domain.OperationStats) error {
	if slug == existing.Slug {
		return nil
	}
	taken, found, queryStats, err := s.store.FindBySlug(ctx, slug)
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if found && taken.ID != existing.ID {
		return organizationtenancy.ErrOrganizationSlugConflict
	}
	return nil
}

func (s *Service) SoftDeleteOrganization(ctx context.Context, id int64) (result organizationtenancy.Organization, err error) {
	if id <= 0 {
		return organizationtenancy.Organization{}, organizationtenancy.ErrInvalidOrganization
	}
	err = s.run(ctx, "soft_delete_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, queryStats, err := s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		if organization.IsDeleted() {
			return organizationtenancy.ErrOrganizationAlreadyDeleted
		}
		count, countStats, err := s.store.CountNonDeletedSchools(txCtx, id)
		stats.Add(countStats)
		if err != nil {
			return err
		}
		if count > 0 {
			return &organizationtenancy.OrganizationHasSchoolsError{SchoolCount: count}
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

func (s *Service) RestoreOrganization(ctx context.Context, id int64) (result organizationtenancy.Organization, err error) {
	if id <= 0 {
		return organizationtenancy.Organization{}, organizationtenancy.ErrInvalidOrganization
	}
	err = s.run(ctx, "restore_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		organization, found, queryStats, err := s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		if !organization.IsDeleted() {
			return organizationtenancy.ErrOrganizationNotDeleted
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

func (s *Service) FindOrganization(ctx context.Context, id int64) (result organizationtenancy.Organization, err error) {
	if id <= 0 {
		return organizationtenancy.Organization{}, organizationtenancy.ErrInvalidOrganization
	}
	err = s.run(ctx, "find_organization", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, "")
		stats.Add(queryStats)
		if err == nil && !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindOrganizationForSchoolMutation(ctx context.Context, id int64) (result organizationtenancy.Organization, err error) {
	if id <= 0 {
		return organizationtenancy.Organization{}, organizationtenancy.ErrInvalidOrganization
	}
	err = s.run(ctx, "find_organization_for_school_mutation", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, "SHARE")
		stats.Add(queryStats)
		if err == nil && !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindOrganizationForMutation(ctx context.Context, id int64) (result organizationtenancy.Organization, err error) {
	if id <= 0 {
		return organizationtenancy.Organization{}, invalid("organization ID is required")
	}
	err = s.run(ctx, "find_organization_for_mutation", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err == nil && !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindOrganizationBySlug(ctx context.Context, slug string) (result organizationtenancy.Organization, err error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if err := organizationtenancy.ValidateSlug(slug); err != nil {
		return organizationtenancy.Organization{}, err
	}
	err = s.run(ctx, "find_organization_by_slug", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindBySlug(txCtx, slug)
		stats.Add(queryStats)
		if err == nil && !found {
			return organizationtenancy.ErrOrganizationNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListOrganizations(ctx context.Context) (result []organizationtenancy.Organization, err error) {
	err = s.run(ctx, "list_organizations", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.List(txCtx)
		stats.Add(queryStats)
		return err
	})
	if result == nil {
		result = []organizationtenancy.Organization{}
	}
	return result, err
}

func (s *Service) ListOrganizationsByID(ctx context.Context, ids []int64) (result []organizationtenancy.Organization, err error) {
	if len(ids) == 0 {
		return []organizationtenancy.Organization{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, organizationtenancy.ErrInvalidOrganization
		}
	}
	err = s.run(ctx, "list_organizations_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	if result == nil {
		result = []organizationtenancy.Organization{}
	}
	return result, err
}

func (s *Service) CountOrganizationsByID(ctx context.Context, ids []int64) (result int, err error) {
	if len(ids) == 0 {
		return 0, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return 0, organizationtenancy.ErrInvalidOrganization
		}
	}
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

var _ organizationtenancy.Capability = (*Service)(nil)
