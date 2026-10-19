package application

import (
	"context"
	"errors"
)

func NewErrorMatcher(sentinel error) func(error) bool {
	return func(err error) bool { return errors.Is(err, sentinel) }
}
func NewTransactionPresence(find func(context.Context) (any, bool)) func(context.Context) bool {
	return func(ctx context.Context) bool { _, ok := find(ctx); return ok }
}
func NewDetachedContext(withoutHooks, withoutTransaction func(context.Context) context.Context) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context { return withoutHooks(withoutTransaction(ctx)) }
}
