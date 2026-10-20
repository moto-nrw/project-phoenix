package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

type fakeOwnerStore struct{ engine *fakeEngine }
type ownerTestTransaction struct{}

func (ownerTestTransaction) RunAdmin(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (ownerTestTransaction) RunRead(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (ownerTestTransaction) RunTenant(ctx context.Context, _ int64, fn func(context.Context) error) error {
	return fn(ctx)
}
func newFakeOwner(engine *fakeEngine) *Service {
	return New(fakeOwnerStore{engine}, ownerTestTransaction{}, func(ports.Observation) {})
}
func (s fakeOwnerStore) Create(ctx context.Context, input organizationtenancy.CreateOrganization) (organizationtenancy.Organization, domain.OperationStats, error) {
	value, err := s.engine.Create(ctx, input)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) Update(ctx context.Context, input organizationtenancy.UpdateOrganization) (organizationtenancy.Organization, domain.OperationStats, error) {
	value, err := s.engine.Update(ctx, input)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) CreateSchool(ctx context.Context, input organizationtenancy.CreateSchool) (organizationtenancy.School, domain.OperationStats, error) {
	value, err := s.engine.CreateSchool(ctx, input)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) UpdateSchool(ctx context.Context, input organizationtenancy.UpdateSchool) (organizationtenancy.School, domain.OperationStats, error) {
	value, err := s.engine.UpdateSchool(ctx, input)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) List(ctx context.Context) ([]organizationtenancy.Organization, domain.OperationStats, error) {
	value, err := s.engine.List(ctx)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) ListByIDs(ctx context.Context, ids []int64) ([]organizationtenancy.Organization, domain.OperationStats, error) {
	value, err := s.engine.ListByIDs(ctx, ids)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) CountByIDs(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	value, err := s.engine.CountByIDs(ctx, ids)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) CountSchoolsByID(ctx context.Context, ids []int64) (int, domain.OperationStats, error) {
	value, err := s.engine.CountSchoolsByID(ctx, ids)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) FindByID(ctx context.Context, id int64, lock string) (organizationtenancy.Organization, bool, domain.OperationStats, error) {
	var value organizationtenancy.Organization
	var err error
	switch lock {
	case "UPDATE":
		value, err = s.engine.FindForMutation(ctx, id)
	case "SHARE":
		value, err = s.engine.FindForSchoolMutation(ctx, id)
	default:
		value, err = s.engine.FindByID(ctx, id)
	}
	if errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		return value, false, domain.OperationStats{}, nil
	}
	return value, err == nil, domain.OperationStats{}, err
}
func (s fakeOwnerStore) FindBySlug(ctx context.Context, slug string) (organizationtenancy.Organization, bool, domain.OperationStats, error) {
	value, err := s.engine.FindBySlug(ctx, slug)
	if errors.Is(err, organizationtenancy.ErrOrganizationNotFound) {
		return value, false, domain.OperationStats{}, nil
	}
	return value, err == nil, domain.OperationStats{}, err
}
func (s fakeOwnerStore) CountNonDeletedSchools(_ context.Context, id int64) (int, domain.OperationStats, error) {
	count := 0
	for _, school := range s.engine.schools {
		if school.OrganizationID == id && !school.IsDeleted() {
			count++
		}
	}
	return count, domain.OperationStats{}, nil
}
func (s fakeOwnerStore) LockSchoolChildQuota(context.Context, int64) (domain.OperationStats, error) {
	return domain.OperationStats{}, nil
}
func (s fakeOwnerStore) SetSchoolChildQuota(ctx context.Context, id int64, bundles *int, size int) (organizationtenancy.School, domain.OperationStats, error) {
	var quota *organizationtenancy.ChildQuota
	if bundles != nil {
		quota = &organizationtenancy.ChildQuota{Bundles: *bundles, BundleSize: size}
	}
	value, err := s.engine.SetSchoolChildQuota(ctx, id, quota)
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) SetSchoolDeleted(ctx context.Context, id int64, deleted bool) (domain.OperationStats, error) {
	var err error
	if deleted {
		_, err = s.engine.SoftDeleteSchool(ctx, id)
	} else {
		_, err = s.engine.RestoreSchool(ctx, id)
	}
	return domain.OperationStats{}, err
}
func (s fakeOwnerStore) ListSchools(ctx context.Context, ids []int64, organizationID *int64, state string) ([]organizationtenancy.School, domain.OperationStats, error) {
	var value []organizationtenancy.School
	var err error
	switch {
	case ids != nil:
		value, err = s.engine.ListSchoolsByID(ctx, ids)
	case organizationID != nil:
		value, err = s.engine.ListSchoolsByOrganization(ctx, *organizationID)
	case state == "active":
		value, err = s.engine.ListActiveSchools(ctx)
	case state == "non_deleted":
		value, err = s.engine.ListNonDeletedSchools(ctx)
	case state == "public":
		value, err = s.engine.ListPublicSchools(ctx)
	default:
		value, err = s.engine.ListSchools(ctx)
	}
	return value, domain.OperationStats{}, err
}
func (s fakeOwnerStore) SoftDelete(ctx context.Context, id int64) (domain.OperationStats, error) {
	_, err := s.engine.SoftDelete(ctx, id)
	return domain.OperationStats{}, err
}
func (s fakeOwnerStore) Restore(ctx context.Context, id int64) (domain.OperationStats, error) {
	_, err := s.engine.Restore(ctx, id)
	return domain.OperationStats{}, err
}
func (s fakeOwnerStore) FindSchoolByID(ctx context.Context, id int64, lock string) (organizationtenancy.School, bool, domain.OperationStats, error) {
	value, err := s.engine.FindSchoolByID(ctx, id, lock)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return value, false, domain.OperationStats{}, nil
	}
	return value, err == nil, domain.OperationStats{}, err
}
func (s fakeOwnerStore) FindSchoolBySlug(ctx context.Context, slug string) (organizationtenancy.School, bool, domain.OperationStats, error) {
	value, err := s.engine.FindSchoolBySlug(ctx, slug)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return value, false, domain.OperationStats{}, nil
	}
	return value, err == nil, domain.OperationStats{}, err
}
func (s fakeOwnerStore) FindSchoolByOrganizationAndSlug(ctx context.Context, id int64, slug string) (organizationtenancy.School, bool, domain.OperationStats, error) {
	value, err := s.engine.FindSchoolByOrganizationAndSlug(ctx, id, slug)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return value, false, domain.OperationStats{}, nil
	}
	return value, err == nil, domain.OperationStats{}, err
}
func (s fakeOwnerStore) FindSchoolBySubdomain(ctx context.Context, subdomain string) (organizationtenancy.School, bool, domain.OperationStats, error) {
	value, err := s.engine.FindSchoolBySubdomain(ctx, subdomain)
	if errors.Is(err, organizationtenancy.ErrSchoolNotFound) {
		return value, false, domain.OperationStats{}, nil
	}
	return value, err == nil, domain.OperationStats{}, err
}
