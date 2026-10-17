package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

func (e Engine) CreateSchool(ctx context.Context, input organizationtenancy.CreateSchool) (organizationtenancy.School, error) {
	value, err := e.service.CreateSchool(ctx, domain.CreateSchool{
		OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug, Subdomain: input.Subdomain,
		Active: input.Active, Hidden: input.Hidden, Settings: input.Settings, Address: input.Address,
		City: input.City, Zip: input.Zip, Phone: input.Phone, Email: input.Email,
	})
	return toPublicSchool(value), MapError(err)
}

func (e Engine) UpdateSchool(ctx context.Context, input organizationtenancy.UpdateSchool) (organizationtenancy.School, error) {
	value, err := e.service.UpdateSchool(ctx, domain.UpdateSchool{
		ID: input.ID, OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug, Subdomain: input.Subdomain,
		Active: input.Active, Hidden: input.Hidden, Settings: input.Settings, Address: input.Address,
		City: input.City, Zip: input.Zip, Phone: input.Phone, Email: input.Email,
	})
	return toPublicSchool(value), MapError(err)
}

func (e Engine) SetSchoolChildQuota(ctx context.Context, id int64, quota *organizationtenancy.ChildQuota) (organizationtenancy.School, error) {
	var bundles *int
	bundleSize := 0
	if quota != nil {
		bundles, bundleSize = &quota.Bundles, quota.BundleSize
	}
	value, err := e.service.SetSchoolChildQuota(ctx, id, bundles, bundleSize)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) SoftDeleteSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	value, err := e.service.SoftDeleteSchool(ctx, id)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) RestoreSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	value, err := e.service.RestoreSchool(ctx, id)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) FindSchoolByID(ctx context.Context, id int64, lock string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolByID(ctx, id, lock)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) FindSchoolBySlug(ctx context.Context, slug string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolBySlug(ctx, slug)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) FindSchoolByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolByOrganizationAndSlug(ctx, organizationID, slug)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) FindSchoolBySubdomain(ctx context.Context, subdomain string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolBySubdomain(ctx, subdomain)
	return toPublicSchool(value), MapError(err)
}

func (e Engine) ListSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "")
}

func (e Engine) ListSchoolsByID(ctx context.Context, ids []int64) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, ids, nil, "")
}

func (e Engine) ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, &organizationID, "")
}

func (e Engine) ListNonDeletedSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "non_deleted")
}

func (e Engine) ListActiveSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "active")
}

func (e Engine) ListPublicSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "public")
}

func (e Engine) listSchools(ctx context.Context, ids []int64, organizationID *int64, state string) ([]organizationtenancy.School, error) {
	values, err := e.service.ListSchools(ctx, ids, organizationID, state)
	return toPublicSchools(values), MapError(err)
}

func (e Engine) CountSchoolsByID(ctx context.Context, ids []int64) (int, error) {
	value, err := e.service.CountSchoolsByID(ctx, ids)
	return value, MapError(err)
}

func toPublicSchool(value domain.School) organizationtenancy.School {
	var organization *organizationtenancy.Organization
	if value.Organization != nil {
		converted := toPublic(*value.Organization)
		organization = &converted
	}
	return organizationtenancy.School{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		OrganizationID: value.OrganizationID, Name: value.Name, Slug: value.Slug, Subdomain: value.Subdomain,
		Active: value.Active, Hidden: value.Hidden, DeletedAt: value.DeletedAt, Settings: value.Settings,
		Address: value.Address, City: value.City, Zip: value.Zip, Phone: value.Phone, Email: value.Email,
		ChildQuotaBundles: value.ChildQuotaBundles, ChildQuotaBundleSize: value.ChildQuotaBundleSize,
		Organization: organization,
	}
}

func toPublicSchools(values []domain.School) []organizationtenancy.School {
	result := make([]organizationtenancy.School, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicSchool(value))
	}
	return result
}
