// Package compose wires the Identity & Access module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the Identity & Access module. Every operation runs on the
// caller's ambient transaction when one exists and otherwise opens one for
// the tenant in context, so the guardian-access writes commit with the
// approval that requested them.
func New(dependencies Dependencies) (*identityaccess.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("identity access compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("identity access postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	})
	service := application.New(store, transaction{}, tenant.FromContext, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return identityaccess.NewModule(engine{service: service}), nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return fmt.Errorf("%w: %w", identityaccess.ErrTenantRequired, err)
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

type engine struct{ service *application.Service }

func (e engine) FindAccount(ctx context.Context, id int64) (identityaccess.Account, error) {
	value, err := e.service.FindAccount(ctx, id)
	return identityaccess.Account(value), mapError(err)
}

func (e engine) FindAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error) {
	value, err := e.service.FindAccountByEmail(ctx, email)
	return identityaccess.Account(value), mapError(err)
}

func (e engine) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (identityaccess.GuardianTenantAccess, error) {
	value, err := e.service.GrantGuardianTenantAccess(ctx, accountID)
	return identityaccess.GuardianTenantAccess(value), mapError(err)
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAccountNotFound):
		return identityaccess.ErrAccountNotFound
	case errors.Is(err, domain.ErrTenantRequired):
		return identityaccess.ErrTenantRequired
	case errors.Is(err, domain.ErrGuardianRoleMissing):
		return identityaccess.ErrGuardianRoleMissing
	default:
		return err
	}
}
