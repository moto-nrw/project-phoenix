// Package compose wires the People Directory module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

func New(dependencies Dependencies) (*peopledirectory.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("people directory compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, int64, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, 0, errors.New("people directory postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, 0, fmt.Errorf("people directory postgres: unsupported transaction %T", transaction)
		}
		return tx, tenant.FromContext(ctx), nil
	})
	service := application.New(store, transaction{}, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return peopledirectory.NewModule(engine{service: service}), nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}

func (transaction) RunSavepoint(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, callback)
}

// RunAdminRead always opens its own admin transaction: a tenant transaction
// already in context would keep row-level security on, which is exactly
// what a platform-wide read must escape.
func (transaction) RunAdminRead(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithinAdmin(tenant.ContextWithoutTransaction(ctx), callback)
}

type engine struct{ service *application.Service }

func (e engine) Create(ctx context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	value, err := e.service.Create(ctx, domain.CreatePerson{
		FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return toPublic(value), mapError(err)
}

func (e engine) Update(ctx context.Context, input peopledirectory.UpdatePerson) (peopledirectory.Person, error) {
	value, err := e.service.Update(ctx, domain.UpdatePerson{
		ID: input.ID, FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return toPublic(value), mapError(err)
}

func (e engine) Delete(ctx context.Context, id int64) error {
	return mapError(e.service.Delete(ctx, id))
}

func (e engine) FindByID(ctx context.Context, id int64, lock string) (peopledirectory.Person, error) {
	value, err := e.service.FindByID(ctx, id, lock)
	return toPublic(value), mapError(err)
}

func (e engine) FindByAccount(ctx context.Context, accountID int64) (peopledirectory.Person, error) {
	value, err := e.service.FindByAccount(ctx, accountID)
	return toPublic(value), mapError(err)
}

func (e engine) FindByTag(ctx context.Context, tagID string) (peopledirectory.Person, error) {
	value, err := e.service.FindByTag(ctx, tagID)
	return toPublic(value), mapError(err)
}

func (e engine) ListByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func (e engine) ListAcrossTenantsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListAcrossTenantsByIDs(ctx, ids)
	return toPublicList(values), mapError(err)
}

func (e engine) ListByAccounts(ctx context.Context, accountIDs []int64) ([]peopledirectory.Person, error) {
	values, err := e.service.ListByAccounts(ctx, accountIDs)
	return toPublicList(values), mapError(err)
}

func (e engine) Search(ctx context.Context, filter peopledirectory.PersonFilter) ([]peopledirectory.Person, error) {
	values, err := e.service.Search(ctx, domain.Filter{
		FirstNamePrefix: filter.FirstNamePrefix, LastNamePrefix: filter.LastNamePrefix,
		FullNameContains: filter.FullNameContains, TagID: filter.TagID, AccountIDs: filter.AccountIDs,
		Page: filter.Page, PageSize: filter.PageSize,
	})
	return toPublicList(values), mapError(err)
}

func (e engine) CountByTenant(ctx context.Context) (map[int64]int, error) {
	value, err := e.service.CountByTenant(ctx)
	return value, mapError(err)
}

func (e engine) LinkAccount(ctx context.Context, personID, accountID int64) error {
	return mapError(e.service.LinkAccount(ctx, personID, accountID))
}

func (e engine) UnlinkAccount(ctx context.Context, personID int64) error {
	return mapError(e.service.UnlinkAccount(ctx, personID))
}

func (e engine) LinkTag(ctx context.Context, personID int64, tagID string) error {
	return mapError(e.service.LinkTag(ctx, personID, tagID))
}

func (e engine) UnlinkTag(ctx context.Context, personID int64) error {
	return mapError(e.service.UnlinkTag(ctx, personID))
}

func (e engine) ReleaseTags(ctx context.Context, personIDs []int64) ([]peopledirectory.ReleasedTag, error) {
	values, err := e.service.ReleaseTags(ctx, personIDs)
	result := make([]peopledirectory.ReleasedTag, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.ReleasedTag{PersonID: value.PersonID, TagID: value.TagID})
	}
	return result, mapError(err)
}

func (e engine) RestoreTag(ctx context.Context, personID int64, tagID string) (bool, error) {
	restored, err := e.service.RestoreTag(ctx, personID, tagID)
	return restored, mapError(err)
}

func toPublic(value domain.Person) peopledirectory.Person {
	return peopledirectory.Person{
		ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
		FirstName: value.FirstName, LastName: value.LastName, Birthday: value.Birthday,
		TagID: value.TagID, AccountID: value.AccountID, DeletedAt: value.DeletedAt,
	}
}

func toPublicList(values []domain.Person) []peopledirectory.Person {
	result := make([]peopledirectory.Person, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return peopledirectory.ErrPersonNotFound
	case errors.Is(err, domain.ErrTagConflict):
		return peopledirectory.ErrTagConflict
	case errors.Is(err, domain.ErrAccountConflict):
		return peopledirectory.ErrAccountConflict
	default:
		return err
	}
}
