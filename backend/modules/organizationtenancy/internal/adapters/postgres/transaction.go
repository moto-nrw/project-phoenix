package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// Transaction binds the owner's commands to the ambient tenant runtime.
type Transaction struct{}

func (Transaction) RunAdmin(ctx context.Context, callback func(context.Context) error) error {
	return TransactionRunAdminBinding(ctx, callback, tenant.WithinAdmin)
}
func (Transaction) RunTenant(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	return TransactionRunTenantBinding(ctx, tenantID, callback, tenant.WithinTenant)
}
func (Transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	return TransactionRunReadBinding(ctx, callback, tenant.WithinAdmin)
}
