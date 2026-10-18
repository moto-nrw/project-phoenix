package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantTransaction runs the module's work on the caller's ambient tenant
// transaction when one exists and otherwise opens one for the tenant in
// context; a read without a tenant runs as an admin read.
type TenantTransaction struct{}

func (TenantTransaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (TenantTransaction) RunSavepoint(ctx context.Context, callback func(context.Context) error) error {
	return tenant.WithSavepoint(ctx, callback)
}

func (TenantTransaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}
