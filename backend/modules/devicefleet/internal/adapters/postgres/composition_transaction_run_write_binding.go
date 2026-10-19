package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func TransactionRunWriteBinding(ctx context.Context, callback func(context.Context) error, execute0 func(ctx context.Context, id tenant.TenantID, fn func(context.Context) error) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return errors.Join(devicefleet.ErrTenantRequired, err)
	}
	return execute0(ctx, id, callback)
}
