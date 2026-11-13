package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
)

func NewCallerReader(readCaller func(context.Context) identityaccess.Caller) func(ctx context.Context) domain.Caller {
	return func(ctx context.Context) domain.Caller {
		caller := domain.Caller(readCaller(ctx))
		caller.TenantID = tenant.FromContext(ctx)
		return caller
	}
}
