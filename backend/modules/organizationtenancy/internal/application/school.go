package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

func (s *Service) CreateSchool(ctx context.Context, input domain.CreateSchool) (result domain.School, err error) {
	err = s.run(ctx, "create_school", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireSchoolOrganization(txCtx, input.OrganizationID, stats); err != nil {
			return err
		}
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateSchool(txCtx, input)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateSchool(ctx context.Context, input domain.UpdateSchool) (result domain.School, err error) {
	err = s.run(ctx, "update_school", func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, findErr := s.store.FindSchoolByID(txCtx, input.ID, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrSchoolNotFound
		}
		if existing.IsDeleted() {
			return domain.ErrSchoolAlreadyDeleted
		}
		if input.OrganizationID != existing.OrganizationID {
			if err := s.requireSchoolOrganization(txCtx, input.OrganizationID, stats); err != nil {
				return err
			}
		}
		result, queryStats, err = s.store.UpdateSchool(txCtx, input)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) SoftDeleteSchool(ctx context.Context, id int64) (domain.School, error) {
	return s.changeSchoolDeletion(ctx, id, true)
}

func (s *Service) RestoreSchool(ctx context.Context, id int64) (domain.School, error) {
	return s.changeSchoolDeletion(ctx, id, false)
}

func (s *Service) changeSchoolDeletion(ctx context.Context, id int64, deleted bool) (result domain.School, err error) {
	operation := "restore_school"
	if deleted {
		operation = "soft_delete_school"
	}
	err = s.run(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, findErr := s.store.FindSchoolByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrSchoolNotFound
		}
		if deleted && existing.IsDeleted() {
			return domain.ErrSchoolAlreadyDeleted
		}
		if !deleted && !existing.IsDeleted() {
			return domain.ErrSchoolNotDeleted
		}
		if !deleted {
			if err := s.requireSchoolOrganization(txCtx, existing.OrganizationID, stats); err != nil {
				return err
			}
		}
		changeStats, changeErr := s.store.SetSchoolDeleted(txCtx, id, deleted)
		stats.Add(changeStats)
		if changeErr != nil {
			return changeErr
		}
		result, _, queryStats, err = s.store.FindSchoolByID(txCtx, id, "")
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) requireSchoolOrganization(ctx context.Context, organizationID int64, stats *domain.OperationStats) error {
	organization, found, queryStats, err := s.store.FindByID(ctx, organizationID, "SHARE")
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrNotFound
	}
	if organization.IsDeleted() {
		return domain.ErrOrganizationDeleted
	}
	return nil
}

func (s *Service) FindSchoolByID(ctx context.Context, id int64, lock string) (result domain.School, err error) {
	run := s.runRead
	if lock != "" {
		run = s.run
	}
	err = run(ctx, "find_school", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindSchoolByID(txCtx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrSchoolNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindSchoolBySlug(ctx context.Context, slug string) (result domain.School, err error) {
	return s.findSchool(ctx, "find_school_by_slug", func(txCtx context.Context) (domain.School, bool, domain.OperationStats, error) {
		return s.store.FindSchoolBySlug(txCtx, slug)
	})
}

func (s *Service) FindSchoolByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (result domain.School, err error) {
	return s.findSchool(ctx, "find_school_by_organization_and_slug", func(txCtx context.Context) (domain.School, bool, domain.OperationStats, error) {
		return s.store.FindSchoolByOrganizationAndSlug(txCtx, organizationID, slug)
	})
}

func (s *Service) FindSchoolBySubdomain(ctx context.Context, subdomain string) (result domain.School, err error) {
	return s.findSchool(ctx, "find_school_by_subdomain", func(txCtx context.Context) (domain.School, bool, domain.OperationStats, error) {
		return s.store.FindSchoolBySubdomain(txCtx, subdomain)
	})
}

func (s *Service) findSchool(ctx context.Context, operation string, query func(context.Context) (domain.School, bool, domain.OperationStats, error)) (result domain.School, err error) {
	err = s.runRead(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = query(txCtx)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrSchoolNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListSchools(ctx context.Context, ids []int64, organizationID *int64, state string) (result []domain.School, err error) {
	err = s.runRead(ctx, "list_schools", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListSchools(txCtx, ids, organizationID, state)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountSchoolsByID(ctx context.Context, ids []int64) (result int, err error) {
	err = s.runRead(ctx, "count_schools_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountSchoolsByID(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
