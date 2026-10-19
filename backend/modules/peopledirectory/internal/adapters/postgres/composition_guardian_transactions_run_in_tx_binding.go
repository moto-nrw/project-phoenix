package postgres

import (
	"context"
)

func GuardianTransactionsRunInTxBinding(ctx context.Context, fn func(context.Context) error, execute0 func(ctx context.Context, fn func(context.Context) error) error) error {
	return execute0(ctx, fn)
}
