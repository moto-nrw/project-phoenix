package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func EngineRuntimeWithinTenantBinding(ctx context.Context, command func(context.Context) error, execute0 func(ctx context.Context, id tenant.TenantID, fn func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return command(ctx)
	}
	tenantID := tenant.FromContext(ctx)
	validatedTenantID, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return errors.New("care plan: tenant is required for commands")
	}
	return execute0(ctx, validatedTenantID, command)
}
