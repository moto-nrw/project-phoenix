package postgres

import (
	"context"
)

func TenantRuntimeWithAdminTxBinding(ctx context.Context, fn func(context.Context) error, execute0 func(ctx context.Context, fn func(context.Context) error) error, execute1 func(context.Context) context.Context) error {
	return execute0(execute1(ctx), fn)
}
