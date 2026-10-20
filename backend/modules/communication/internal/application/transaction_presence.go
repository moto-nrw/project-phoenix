package application

import "context"

func TransactionPresent(ctx context.Context, current func(context.Context) (any, bool)) bool {
	_, ok := current(ctx)
	return ok
}
