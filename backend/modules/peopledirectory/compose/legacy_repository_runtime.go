package compose

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// LegacyRepositoryRuntime binds the retained People Directory repositories
// (database/repositories/users) to the shared tenant runtime, so those
// repositories read neither the tenant context nor the transaction protocol
// themselves (#2727). The legacy composition that adapts those repositories
// reads the caller's school through it as well.
type LegacyRepositoryRuntime struct{ db *bun.DB }

// NewLegacyRepositoryRuntime returns the runtime over db, the pool used
// outside a tenant transaction.
func NewLegacyRepositoryRuntime(db *bun.DB) LegacyRepositoryRuntime {
	if db == nil {
		panic("people directory compose: database is required")
	}
	return LegacyRepositoryRuntime{db: db}
}

// DB is the ambient tenant transaction, or the pool outside one.
func (r LegacyRepositoryRuntime) DB(ctx context.Context) bun.IDB {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return r.db
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx
	case *bun.Tx:
		if tx != nil {
			return tx
		}
	}
	panic(fmt.Sprintf("people directory compose: unsupported transaction type %T", transaction))
}

// TenantID is the school the caller acts for, 0 when none.
func (LegacyRepositoryRuntime) TenantID(ctx context.Context) int64 { return CallerTenantID(ctx) }

// CallerTenantID is the school the caller acts for, 0 when none. The legacy
// composition that adapts the retained repositories reads it here.
func CallerTenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// WithCallerTenant scopes ctx to one school for the legacy composition.
func WithCallerTenant(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

// RequireTenantID is TenantID, failing with tenant.ErrTenantRequired when the
// context carries none.
func (LegacyRepositoryRuntime) RequireTenantID(ctx context.Context) (int64, error) {
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return id.Int64(), nil
}

// InTransaction reports whether ctx carries a tenant transaction.
func (LegacyRepositoryRuntime) InTransaction(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

// RunInTx joins the ambient transaction or opens one for the context's tenant.
func (LegacyRepositoryRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// WithSavepoint runs fn behind a savepoint of the ambient transaction.
func (LegacyRepositoryRuntime) WithSavepoint(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, fn)
}

// WithinCurrentTenant opens a transaction for the context's tenant.
func (LegacyRepositoryRuntime) WithinCurrentTenant(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinCurrentTenant(ctx, fn)
}
