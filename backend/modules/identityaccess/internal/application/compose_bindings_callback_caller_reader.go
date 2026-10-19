package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewCallerReader(readCaller func(context.Context) identityaccess.Caller) func(ctx context.Context) domain.Caller {
	return func(ctx context.Context) domain.Caller {
		caller := domain.Caller(readCaller(ctx))
		caller.TenantID = tenant.FromContext(ctx)
		return caller
	}
}
