package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// RepositoryRuntime provides the context-only tenant capabilities of the owner.
type RepositoryRuntime struct{}

// TenantID is the school the caller acts for, 0 when none.
func (RepositoryRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// RequireTenantID is TenantID, failing with tenant.ErrTenantRequired when the
// context carries none.
func (RepositoryRuntime) RequireTenantID(ctx context.Context) (int64, error) {
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, err
	}
	return id.Int64(), nil
}

// InTransaction reports whether ctx carries a tenant transaction.
func (RepositoryRuntime) InTransaction(ctx context.Context) bool {
	_, ok := tenant.TransactionFromContext(ctx)
	return ok
}

// WithSavepoint runs fn behind a savepoint of the ambient transaction.
func (RepositoryRuntime) WithSavepoint(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, fn)
}

// MarkRollback asks the request's tenant transaction to roll back although
// the handler answers.
func (RepositoryRuntime) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }

// AcrossSchools is available only in an explicit administrative transaction.
func (RepositoryRuntime) AcrossSchools(ctx context.Context) bool {
	_, err := tenant.TenantFromContext(ctx)
	return err != nil && tenant.IsAdminTx(ctx)
}

// InTenantTx belongs to the application boundary, not the database resolver.
func (RepositoryRuntime) InTenantTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}
