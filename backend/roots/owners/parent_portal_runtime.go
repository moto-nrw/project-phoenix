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

// WithinAdminThenTenant forwards the one-way handoff of ADR 0161 unchanged:
// one transaction, discovery as phoenix_admin, then fn under the discovered
// school's RLS.
func (ParentPortalRuntime) WithinAdminThenTenant(ctx context.Context, discover func(context.Context) (int64, error), fn func(context.Context) error) error {
	return tenant.WithinAdminThenTenant(ctx, discover, fn)
}

// AmbientScope reports whether a unit of any scope (school, administrative,
// Träger) is open in ctx, and the school ctx is bound to (0 for none): the
// two facts that decide whether a bundled unit could start without joining
// or refusing a foreign one (ADR 0161 point 7).
func (ParentPortalRuntime) AmbientScope(ctx context.Context) (bool, int64) {
	_, open := tenant.TransactionFromContext(ctx)
	return open || tenant.IsAdminTx(ctx) || tenant.TraegerTransactionFromContext(ctx) != 0, tenant.FromContext(ctx)
}

func (ParentPortalRuntime) RegisterAfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (ParentPortalRuntime) WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}
