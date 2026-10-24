package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

func (s *Service) CreateSchool(ctx context.Context, input organizationtenancy.CreateSchool) (result organizationtenancy.School, err error) {
	organizationtenancy.NormalizeSchool(&input.Name, &input.Slug, &input.Subdomain)
	if err := organizationtenancy.ValidateSchool(input.OrganizationID, input.Name, input.Slug, input.Subdomain, input.Email); err != nil {
		return organizationtenancy.School{}, err
	}
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

func (s *Service) UpdateSchool(ctx context.Context, input organizationtenancy.UpdateSchool) (result organizationtenancy.School, err error) {
	organizationtenancy.NormalizeSchool(&input.Name, &input.Slug, &input.Subdomain)
	if input.ID <= 0 {
		return organizationtenancy.School{}, invalidSchool("school ID is required")
	}
	if err := organizationtenancy.ValidateSchool(input.OrganizationID, input.Name, input.Slug, input.Subdomain, input.Email); err != nil {
		return organizationtenancy.School{}, err
	}
	err = s.run(ctx, "update_school", func(txCtx context.Context, stats *domain.OperationStats) error {
		existing, found, queryStats, findErr := s.store.FindSchoolByID(txCtx, input.ID, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return organizationtenancy.ErrSchoolNotFound
		}
		if existing.IsDeleted() {
			return organizationtenancy.ErrSchoolAlreadyDeleted
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

// SetSchoolChildQuota replaces a live school's Kinderkontingent (#3567).
func (s *Service) SetSchoolChildQuota(ctx context.Context, id int64, quota *organizationtenancy.ChildQuota) (result organizationtenancy.School, err error) {
	if id <= 0 {
		return organizationtenancy.School{}, invalidSchool("school ID is required")
	}
	if quota != nil {
		if err := quota.Validate(); err != nil {
			return organizationtenancy.School{}, err
		}
	}
	var bundles *int
	bundleSize := 0
	if quota != nil {
		bundles, bundleSize = &quota.Bundles, quota.BundleSize
	}
	err = s.run(ctx, "set_school_child_quota", func(txCtx context.Context, stats *domain.OperationStats) error {
		lockStats, lockErr := s.store.LockSchoolChildQuota(txCtx, id)
		stats.Add(lockStats)
		if lockErr != nil {
			return lockErr
		}
		existing, found, queryStats, findErr := s.store.FindSchoolByID(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return organizationtenancy.ErrSchoolNotFound
		}
		if existing.IsDeleted() {
			return organizationtenancy.ErrSchoolAlreadyDeleted
		}
		result, queryStats, err = s.store.SetSchoolChildQuota(txCtx, id, bundles, bundleSize)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) SoftDeleteSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	if id <= 0 {
		return organizationtenancy.School{}, invalidSchool("school ID is required")
	}
	return s.changeSchoolDeletion(ctx, id, true)
}

func (s *Service) RestoreSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	if id <= 0 {
		return organizationtenancy.School{}, invalidSchool("school ID is required")
	}
	return s.changeSchoolDeletion(ctx, id, false)
}

func (s *Service) changeSchoolDeletion(ctx context.Context, id int64, deleted bool) (result organizationtenancy.School, err error) {
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
			return organizationtenancy.ErrSchoolNotFound
		}
		if err := s.requireSchoolDeletionChange(txCtx, existing, deleted, stats); err != nil {
			return err
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
		return organizationtenancy.ErrOrganizationNotFound
	}
	if organization.IsDeleted() {
		return organizationtenancy.ErrOrganizationDeleted
	}
	return nil
}

func (s *Service) findSchoolByID(ctx context.Context, id int64, lock string) (result organizationtenancy.School, err error) {
	if id <= 0 {
		return organizationtenancy.School{}, invalidSchool("school ID is required")
	}
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
			return organizationtenancy.ErrSchoolNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindSchoolBySlug(ctx context.Context, slug string) (result organizationtenancy.School, err error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if err := organizationtenancy.ValidateSlug(slug); err != nil {
		return organizationtenancy.School{}, invalidSchool(err.Error())
	}
	return s.findSchool(ctx, "find_school_by_slug", func(txCtx context.Context) (organizationtenancy.School, bool, domain.OperationStats, error) {
		return s.store.FindSchoolBySlug(txCtx, slug)
	})
}

func (s *Service) FindSchoolByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (result organizationtenancy.School, err error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if organizationID <= 0 {
		return organizationtenancy.School{}, invalidSchool("organization ID is required")
	}
	if err := organizationtenancy.ValidateSlug(slug); err != nil {
		return organizationtenancy.School{}, invalidSchool(err.Error())
	}
	return s.findSchool(ctx, "find_school_by_organization_and_slug", func(txCtx context.Context) (organizationtenancy.School, bool, domain.OperationStats, error) {
		return s.store.FindSchoolByOrganizationAndSlug(txCtx, organizationID, slug)
	})
}

func (s *Service) FindSchoolBySubdomain(ctx context.Context, subdomain string) (result organizationtenancy.School, err error) {
	subdomain = strings.TrimSpace(strings.ToLower(subdomain))
	if err := organizationtenancy.ValidateSubdomain(subdomain); err != nil {
		return organizationtenancy.School{}, err
	}
	return s.findSchool(ctx, "find_school_by_subdomain", func(txCtx context.Context) (organizationtenancy.School, bool, domain.OperationStats, error) {
		return s.store.FindSchoolBySubdomain(txCtx, subdomain)
	})
}

func (s *Service) findSchool(ctx context.Context, operation string, query func(context.Context) (organizationtenancy.School, bool, domain.OperationStats, error)) (result organizationtenancy.School, err error) {
	err = s.runRead(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = query(txCtx)
		stats.Add(queryStats)
		if err == nil && !found {
			return organizationtenancy.ErrSchoolNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) listSchools(ctx context.Context, ids []int64, organizationID *int64, state string) (result []organizationtenancy.School, err error) {
	err = s.runRead(ctx, "list_schools", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListSchools(txCtx, ids, organizationID, state)
		stats.Add(queryStats)
		return err
	})
	if result == nil {
		result = []organizationtenancy.School{}
	}
	return result, err
}

func (s *Service) CountSchoolsByID(ctx context.Context, ids []int64) (result int, err error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if err := validateSchoolIDs(ids); err != nil {
		return 0, err
	}
	err = s.runRead(ctx, "count_schools_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountSchoolsByID(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) requireSchoolDeletionChange(ctx context.Context, existing organizationtenancy.School, deleted bool, stats *domain.OperationStats) error {
	if deleted && existing.IsDeleted() {
		return organizationtenancy.ErrSchoolAlreadyDeleted
	}
	if !deleted && !existing.IsDeleted() {
		return organizationtenancy.ErrSchoolNotDeleted
	}
	if !deleted {
		if err := s.requireSchoolOrganization(ctx, existing.OrganizationID, stats); err != nil {
			return err
		}
	}
	return nil
}
