package application

import "context"

func NewRecoveryRuntime(execute func(context.Context, func(context.Context) error) error, withoutHooks func(context.Context) context.Context, withoutTransaction func(context.Context) context.Context) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, fn func(context.Context) error) error {
		return execute(withoutHooks(withoutTransaction(ctx)), fn)
	}
}
