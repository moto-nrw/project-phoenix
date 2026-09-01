package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

func New(dependencies Dependencies) (*organizationtenancy.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("organization tenancy compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("organization tenancy postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("organization tenancy postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	})
	service := application.New(store, transaction{}, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return organizationtenancy.NewModule(engine{service: service}), nil
}

type transaction struct{}

func (transaction) RunAdmin(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, callback)
}

type engine struct{ service *application.Service }

func (e engine) Create(ctx context.Context, input organizationtenancy.CreateOrganization) (organizationtenancy.Organization, error) {
	value, err := e.service.Create(ctx, domain.CreateOrganization{Name: input.Name, Slug: input.Slug, Active: input.Active})
	return toPublic(value), mapError(err)
}

func (e engine) Update(ctx context.Context, input organizationtenancy.UpdateOrganization) (organizationtenancy.Organization, error) {
	value, err := e.service.Update(ctx, domain.UpdateOrganization{ID: input.ID, Name: input.Name, Slug: input.Slug, Active: input.Active})
	return toPublic(value), mapError(err)
}

func (e engine) SoftDelete(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.SoftDelete(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) Restore(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.Restore(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) FindByID(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) FindBySlug(ctx context.Context, slug string) (organizationtenancy.Organization, error) {
	value, err := e.service.FindBySlug(ctx, slug)
	return toPublic(value), mapError(err)
}

func (e engine) List(ctx context.Context) ([]organizationtenancy.Organization, error) {
	values, err := e.service.List(ctx)
	return toPublicList(values), mapError(err)
}

func (e engine) ListByIDs(ctx context.Context, ids []int64) ([]organizationtenancy.Organization, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func (e engine) CountByIDs(ctx context.Context, ids []int64) (int, error) {
	value, err := e.service.CountByIDs(ctx, ids)
	return value, mapError(err)
}

func (e engine) FindForSchoolMutation(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.FindForSchoolMutation(ctx, id)
	return toPublic(value), mapError(err)
}

func (e engine) FindForMutation(ctx context.Context, id int64) (organizationtenancy.Organization, error) {
	value, err := e.service.FindForMutation(ctx, id)
	return toPublic(value), mapError(err)
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

func mapError(err error) error {
	switch {
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
