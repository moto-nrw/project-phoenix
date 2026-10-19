package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TransactionRunTenantBinding(ctx context.Context, tenantID int64, callback func(context.Context) error, execute0 func(ctx context.Context, id tenant.TenantID, fn func(context.Context) error) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return execute0(ctx, id, callback)
}
