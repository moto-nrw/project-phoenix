package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	app "github.com/moto-nrw/project-phoenix/modules/dataimport/internal/adapters/runtimebinding"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTransactions binds the import execution port to the tenant runtime.
func NewTransactions() dataimport.Transactions { return transactions{} }

type transactions struct{}

func (transactions) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (transactions) HasTransaction(ctx context.Context) bool {
	return app.ImportTransactionPresent(ctx, tenant.TransactionFromContext)
}

func (transactions) WithinCurrent(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinCurrentTenant(ctx, fn)
}

func (transactions) WithinRetry(ctx context.Context, fn func(context.Context) error) error {
	return app.RetryImportTransaction(ctx, fn, tenant.TenantFromContext, tenant.WithinTenantRetry)
}

func (transactions) Savepoint(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, fn)
}

func (transactions) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

func (transactions) Observe(ctx context.Context, observe func(dataimport.TransactionObservation)) context.Context {
	return tenant.WithAdditionalUnitOfWorkObserver(ctx, func(event tenant.UnitOfWorkEvent) {
		app.ObserveImportTransaction(observe, event.Kind, event.Duration, event.Retries, tenant.UnitOfWorkPoolWait, tenant.UnitOfWorkLockWait, tenant.UnitOfWorkTransaction)
	})
}
