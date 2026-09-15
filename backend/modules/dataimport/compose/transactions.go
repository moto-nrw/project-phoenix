package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTransactions binds the import execution port to the tenant runtime.
func NewTransactions() dataimport.Transactions { return transactions{} }

type transactions struct{}

func (transactions) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (transactions) HasTransaction(ctx context.Context) bool {
	_, active := tenant.TransactionFromContext(ctx)
	return active
}

func (transactions) WithinCurrent(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinCurrentTenant(ctx, fn)
}

func (transactions) WithinRetry(ctx context.Context, fn func(context.Context) error) error {
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return tenant.WithinTenantRetry(ctx, id, fn)
}

func (transactions) Savepoint(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, fn)
}

func (transactions) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

func (transactions) Observe(ctx context.Context, observe func(dataimport.TransactionObservation)) context.Context {
	return tenant.WithAdditionalUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		var observation dataimport.TransactionObservation
		switch event.Kind {
		case tenant.UnitOfWorkPoolWait:
			observation.PoolWait = event.Duration
		case tenant.UnitOfWorkLockWait:
			observation.LockWait = event.Duration
		case tenant.UnitOfWorkTransaction:
			observation.Retries = event.Retries
		default:
			return
		}
		observe(observation)
	})
}
