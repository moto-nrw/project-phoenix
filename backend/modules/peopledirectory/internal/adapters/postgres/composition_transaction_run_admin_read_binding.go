package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func TransactionRunAdminReadBinding(ctx context.Context, callback func(context.Context) error, execute0 func(ctx context.Context, fn func(context.Context) error) error) error {
	return execute0(tenant.ContextWithoutTransaction(ctx), callback)
}
