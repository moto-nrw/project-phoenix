package postgres

import (
	"context"
)

func TransactionRunReadBinding(ctx context.Context, callback func(context.Context) error, execute0 func(ctx context.Context, callback func(context.Context) error) error) error {
	return execute0(ctx, callback)
}
