package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

// NewEngine serves the capability from the application service.
func NewEngine(service *Service) Engine { return Engine{service: service} }

// Engine is the capability over the application service.
type Engine struct{ service *Service }

func (e Engine) Create(ctx context.Context, input organizationtenancy.CreateOrganization) (organizationtenancy.Organization, error) {
	value, err := e.service.Create(ctx, domain.CreateOrganization{Name: input.Name, Slug: input.Slug, Active: input.Active})
	return toPublic(value), MapError(err)
}

func (e Engine) Update(ctx context.Context, input organizationtenancy.UpdateOrganization) (organizationtenancy.Organization, error) {
	value, err := e.service.Update(ctx, domain.UpdateOrganization{ID: input.ID, Name: input.Name, Slug: input.Slug, Active: input.Active})
	return toPublic(value), MapError(err)
}

func (e Engine) SoftDelete(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.SoftDelete(ctx, id)
	return toPublic(value), MapError(err)
}

func (e Engine) Restore(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.Restore(ctx, id)
	return toPublic(value), MapError(err)
}

func (e Engine) FindByID(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), MapError(err)
}

func (e Engine) FindBySlug(ctx context.Context, slug string) (organizationtenancy.Organization, error) {
	value, err := e.service.FindBySlug(ctx, slug)
	return toPublic(value), MapError(err)
}

func (e Engine) List(ctx context.Context) ([]organizationtenancy.Organization, error) {
	values, err := e.service.List(ctx)
	return toPublicList(values), MapError(err)
}

func (e Engine) ListByIDs(ctx context.Context, ids []int64) ([]organizationtenancy.Organization, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	return toPublicList(values), MapError(err)
}

func (e Engine) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	value, err := e.service.CountByIDs(ctx, ids)
	return value, MapError(err)
}

func (e Engine) FindForSchoolMutation(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.FindForSchoolMutation(ctx, id)
	return toPublic(value), MapError(err)
}

func (e Engine) FindForMutation(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.FindForMutation(ctx, id)
	return toPublic(value), MapError(err)
}

func toPublic(value domain.Organization) organizationtenancy.Organization {
	return organizationtenancy.Organization{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, Slug: value.Slug, Active: value.Active,
		DeletedAt: value.DeletedAt, Settings: value.Settings,
	}
}

func toPublicList(values []domain.Organization) []organizationtenancy.Organization {
	result := make([]organizationtenancy.Organization, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result
}

func MapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrSchoolNotFound):
		return organizationtenancy.ErrSchoolNotFound
	case errors.Is(err, domain.ErrSchoolSlugConflict):
		return organizationtenancy.ErrSchoolSlugConflict
	case errors.Is(err, domain.ErrSchoolDomainConflict):
		return organizationtenancy.ErrSchoolDomainConflict
	case errors.Is(err, domain.ErrSchoolAlreadyDeleted):
		return organizationtenancy.ErrSchoolAlreadyDeleted
	case errors.Is(err, domain.ErrSchoolNotDeleted):
		return organizationtenancy.ErrSchoolNotDeleted
	case errors.Is(err, domain.ErrOrganizationDeleted):
		return organizationtenancy.ErrOrganizationDeleted
	case errors.Is(err, domain.ErrNotFound):
		return organizationtenancy.ErrOrganizationNotFound
	case errors.Is(err, domain.ErrSlugConflict):
		return organizationtenancy.ErrOrganizationSlugConflict
	case errors.Is(err, domain.ErrAlreadyDeleted):
		return organizationtenancy.ErrOrganizationAlreadyDeleted
	case errors.Is(err, domain.ErrNotDeleted):
		return organizationtenancy.ErrOrganizationNotDeleted
	case errors.Is(err, domain.ErrHasSchools):
		var hasSchools *domain.HasSchoolsError
		if errors.As(err, &hasSchools) {
			return &organizationtenancy.OrganizationHasSchoolsError{SchoolCount: hasSchools.Count}
		}
		return organizationtenancy.ErrOrganizationHasSchools
	default:
		return err
	}
}
