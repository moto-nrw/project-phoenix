package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// ParentPortalRuntime binds the workflow's transaction and callback ports to
// the public tenant runtime carried by the request context.
type ParentPortalRuntime struct{}

func (ParentPortalRuntime) WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}

func (ParentPortalRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, fn)
}

func (ParentPortalRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (ParentPortalRuntime) WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}
