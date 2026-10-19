// Package compose bootstraps the PostgreSQL tenant runtime: it turns a
// *bun.DB into the tenant.UnitOfWork and opens
// the pools it runs on. The composition roots and the shared fixtures build
// the runtime here instead of assembling the adapter themselves (#3970).
package compose

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTenantRuntime binds tenant/admin transactions under their database roles,
// savepoints, advisory locks and retry-safe deadlock/serialization replay.
// It hands context to repositories and Audit; pool waits reach the observer.
func NewTenantRuntime(db *bun.DB) (tenant.UnitOfWork, error) {
	unitOfWork, err := postgres.NewPostgresUnitOfWork(db, tenant.ObservePoolWait)
	return bindTenantRuntime(unitOfWork, err, postgres.IsRetryableTransactionError, unitOfWork.WithinAdmin)
}

// NewDemoTenantRuntime retains phoenix_demo for school and global queue/expiry
// units. It never assumes phoenix_admin and does not replay commands.
func NewDemoTenantRuntime(db *bun.DB) (tenant.UnitOfWork, error) {
	unitOfWork, err := postgres.NewDemoPostgresUnitOfWork(db, func(context.Context, time.Duration) {})
	return bindTenantRuntime(unitOfWork, err, func(error) bool { return false }, unitOfWork.WithinDemo)
}

func bindTenantRuntime(unitOfWork *postgres.PostgresUnitOfWork, err error, retryable func(error) bool, global func(context.Context, func(context.Context, any) error) error) (tenant.UnitOfWork, error) {
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	runtime, err := tenant.NewUnitOfWork(
		unitOfWork.WithinTenant,
		global,
		tenant.SavepointFunc(unitOfWork),
		retryable,
		unitOfWork.AcquireLock,
	)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	runtime = runtime.WithTransactionDetacher(unitOfWork.ContextWithoutTransaction)
	return runtime.WithContextAdapters(auditlog.ContextWithTenant, auditlog.ContextWithTransaction), nil
}

// ObserveLockWaits reports the lock wait of every statement on db to the
// unit-of-work observer of the statement's context.
func ObserveLockWaits(db *bun.DB) {
	database.AttachLockWaitHook(db, tenant.ObserveLockWait)
}

// OpenPool opens the superuser pool on the configured database DSN.
func OpenPool() (*bun.DB, error) { return database.DBConn() }

// OpenServePool opens the HTTP server's least-privilege phoenix_auth pool.
func OpenServePool() (*bun.DB, error) { return database.DBConnForServe() }

// TenantCallback binds a context-only callback to the retained SQL type gate.
func TenantCallback(fn func(context.Context) error) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, _ bun.Tx) error { return fn(ctx) }
}
