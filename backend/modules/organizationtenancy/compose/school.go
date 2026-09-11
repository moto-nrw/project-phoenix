package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

func (e engine) CreateSchool(ctx context.Context, input organizationtenancy.CreateSchool) (organizationtenancy.School, error) {
	value, err := e.service.CreateSchool(ctx, domain.CreateSchool{
		OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug, Subdomain: input.Subdomain,
		Active: input.Active, Hidden: input.Hidden, Settings: input.Settings, Address: input.Address,
		City: input.City, Zip: input.Zip, Phone: input.Phone, Email: input.Email, DevicePinHash: input.DevicePinHash,
	})
	return toPublicSchool(value), mapError(err)
}

func (e engine) UpdateSchool(ctx context.Context, input organizationtenancy.UpdateSchool) (organizationtenancy.School, error) {
	value, err := e.service.UpdateSchool(ctx, domain.UpdateSchool{
		ID: input.ID, OrganizationID: input.OrganizationID, Name: input.Name, Slug: input.Slug, Subdomain: input.Subdomain,
		Active: input.Active, Hidden: input.Hidden, Settings: input.Settings, Address: input.Address,
		City: input.City, Zip: input.Zip, Phone: input.Phone, Email: input.Email, DevicePinHash: input.DevicePinHash,
	})
	return toPublicSchool(value), mapError(err)
}

func (e engine) SoftDeleteSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	value, err := e.service.SoftDeleteSchool(ctx, id)
	return toPublicSchool(value), mapError(err)
}

func (e engine) RestoreSchool(ctx context.Context, id int64) (organizationtenancy.School, error) {
	value, err := e.service.RestoreSchool(ctx, id)
	return toPublicSchool(value), mapError(err)
}

func (e engine) FindSchoolByID(ctx context.Context, id int64, lock string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolByID(ctx, id, lock)
	return toPublicSchool(value), mapError(err)
}

func (e engine) FindSchoolBySlug(ctx context.Context, slug string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolBySlug(ctx, slug)
	return toPublicSchool(value), mapError(err)
}

func (e engine) FindSchoolByOrganizationAndSlug(ctx context.Context, organizationID int64, slug string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolByOrganizationAndSlug(ctx, organizationID, slug)
	return toPublicSchool(value), mapError(err)
}

func (e engine) FindSchoolBySubdomain(ctx context.Context, subdomain string) (organizationtenancy.School, error) {
	value, err := e.service.FindSchoolBySubdomain(ctx, subdomain)
	return toPublicSchool(value), mapError(err)
}

func (e engine) ListSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "")
}

func (e engine) ListSchoolsByID(ctx context.Context, ids []int64) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, ids, nil, "")
}

func (e engine) ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, &organizationID, "")
}

func (e engine) ListNonDeletedSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "non_deleted")
}

func (e engine) ListActiveSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "active")
}

func (e engine) ListPublicSchools(ctx context.Context) ([]organizationtenancy.School, error) {
	return e.listSchools(ctx, nil, nil, "public")
}

func (e engine) listSchools(ctx context.Context, ids []int64, organizationID *int64, state string) ([]organizationtenancy.School, error) {
	values, err := e.service.ListSchools(ctx, ids, organizationID, state)
	return toPublicSchools(values), mapError(err)
}

func (e engine) CountSchoolsByID(ctx context.Context, ids []int64) (int, error) {
	value, err := e.service.CountSchoolsByID(ctx, ids)
	return value, mapError(err)
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
		DevicePinHash: value.DevicePinHash, Organization: organization,
	}
}

func toPublicSchools(values []domain.School) []organizationtenancy.School {
	result := make([]organizationtenancy.School, 0, len(values))
	for _, value := range values {
		result = append(result, toPublicSchool(value))
	}
	return result
}
