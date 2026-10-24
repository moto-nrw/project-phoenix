package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewTenantID() func(ctx context.Context) (int64, error) {
	return func(ctx context.Context) (int64, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return 0, err
		}
		return id.Int64(), nil
	}
}
