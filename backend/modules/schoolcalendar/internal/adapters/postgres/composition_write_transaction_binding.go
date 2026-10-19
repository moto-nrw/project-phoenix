package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func WriteTransactionBinding(ctx context.Context, fn func(context.Context) error, execute0 func(ctx context.Context, fn func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return fn(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return fmt.Errorf("school calendar: tenant is required: %w", err)
	}
	return execute0(ctx, fn)
}
