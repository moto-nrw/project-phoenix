package application

import (
	"context"
	"errors"
)

func RequireTransaction[I any](ctx context.Context, current func(context.Context) (any, bool), school func(context.Context) (I, error)) error {
	if _, ok := current(ctx); !ok {
		return errors.New("device scan requires a tenant transaction")
	}
	_, err := school(ctx)
	return err
}
