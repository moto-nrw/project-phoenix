// Package compose bootstraps the PostgreSQL tenant runtime: it turns a
// *bun.DB into the tenant.UnitOfWork that units of work run under, and opens
// the pools it runs on. The composition roots and the shared fixtures build
// the runtime here instead of assembling the adapter themselves (#3970).
package compose

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewTenantRuntime binds the PostgreSQL unit of work on db: tenant and
// administrative transactions under their database roles, savepoints,
// advisory locks, the replay of deadlock and serialization failures for
// retry-safe commands, and the context handoff to the repositories and the
// Audit adapters. Pool waits reach the unit-of-work observer.
func NewTenantRuntime(db *bun.DB) (tenant.UnitOfWork, error) {
	unitOfWork, err := database.NewPostgresUnitOfWork(db, tenant.ObservePoolWait)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	return bindTenantRuntime(unitOfWork, database.IsRetryableTransactionError)
}

// NewDemoTenantRuntime is NewTenantRuntime for the standing demo's
// restricted database role: tenant transactions keep RLS without switching to
// phoenix_tenant, no command is replayed and pool waits are not observed.
func NewDemoTenantRuntime(db *bun.DB) (tenant.UnitOfWork, error) {
	unitOfWork, err := database.NewDemoPostgresUnitOfWork(db, func(context.Context, time.Duration) {})
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	return bindTenantRuntime(unitOfWork, func(error) bool { return false })
}

func bindTenantRuntime(unitOfWork *database.PostgresUnitOfWork, retryable func(error) bool) (tenant.UnitOfWork, error) {
	runtime, err := tenant.NewUnitOfWork(
		unitOfWork.WithinTenant,
		unitOfWork.WithinAdmin,
		tenant.SavepointFunc(unitOfWork),
		retryable,
		unitOfWork.AcquireLock,
	)
	if err != nil {
		return tenant.UnitOfWork{}, err
	}
	runtime = runtime.WithTransactionDetacher(unitOfWork.ContextWithoutTransaction)
	runtime = runtime.WithContextAdapters(unitOfWork.ContextWithTenant, unitOfWork.ContextWithTransaction)
	return runtime.WithContextAdapters(auditlog.ContextWithTenant, auditlog.ContextWithTransaction), nil
}

// ObserveLockWaits reports the lock wait of every statement on db to the
// unit-of-work observer of the statement's context.
func ObserveLockWaits(db *bun.DB) {
	database.AttachLockWaitHook(db, tenant.ObserveLockWait)
}

// OpenPool opens the superuser pool on the configured database DSN.
func OpenPool() (*bun.DB, error) { return database.DBConn() }

// OpenServePool opens the least-privilege phoenix_auth pool the HTTP server
// runs on.
func OpenServePool() (*bun.DB, error) { return database.DBConnForServe() }
