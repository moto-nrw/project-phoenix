package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TransactionRunReadBinding(ctx context.Context, callback func(context.Context) error, execute0 func(ctx context.Context, fn func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return execute0(ctx, callback)
}
