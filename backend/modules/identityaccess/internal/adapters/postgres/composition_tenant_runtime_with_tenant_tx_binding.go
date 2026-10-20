package postgres

import (
	"context"
)

func TenantRuntimeWithTenantTxBinding(ctx context.Context, tenantID int64, fn func(context.Context) error, execute0 func(ctx context.Context, _ struct{}, rawID int64, fn func(context.Context, any) error) error, execute1 func(context.Context) context.Context) error {
	return execute0(execute1(ctx), struct{}{}, tenantID, func(txCtx context.Context, _ any) error {
		return fn(txCtx)
	})
}
