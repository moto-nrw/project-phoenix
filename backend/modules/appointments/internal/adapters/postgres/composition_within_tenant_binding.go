package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func WithinTenantBinding(ctx context.Context, command func(context.Context) error, execute0 func(ctx context.Context, id tenant.TenantID, fn func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return errors.New("appointments: tenant is required for commands")
	}
	return execute0(ctx, tenantID, command)
}
