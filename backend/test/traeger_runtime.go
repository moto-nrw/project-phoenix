package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// WithinTraeger runs fn in the Träger transaction of organizationID on the
// unit of work in ctx (ContextWithTenantRuntime). It serves tests whose role
// may not import the tenant runtime, such as the race tests of the
// Betreuungsvertrag's conclusion (#4680).
func WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return tenant.WithinTraeger(ctx, organizationID, fn)
}
