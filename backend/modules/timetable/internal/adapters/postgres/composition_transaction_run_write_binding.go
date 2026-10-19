package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TransactionRunWriteBinding(ctx context.Context, retry bool, callback func(context.Context) error, execute0 func(ctx context.Context, fn func(context.Context) error) error, execute1 func(ctx context.Context, id tenant.TenantID, fn func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if !retry {
		if _, err := tenant.TenantFromContext(ctx); err != nil {
			return fmt.Errorf("timetable postgres: tenant is required: %w", err)
		}
		return execute0(ctx, callback)
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return execute1(ctx, tenantID, callback)
}
