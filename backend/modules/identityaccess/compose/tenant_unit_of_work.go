package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// TenantUnitOfWork is the tenant runtime the account routes of
// modules/identityaccess/inbound/account run in (#2736): the transaction
// for the tenant in context or the administrative one, the rollback marker
// of the tenant transaction middleware, and the tenant of the request.
type TenantUnitOfWork struct{}

// WithinCurrentTenant runs fn in the transaction of the tenant in ctx.
func (TenantUnitOfWork) WithinCurrentTenant(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinCurrentTenant(ctx, fn)
}

// WithinAdmin runs fn in the administrative transaction.
func (TenantUnitOfWork) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, fn)
}

// MarkRollback rolls the request's tenant transaction back although the
// handler answers.
func (TenantUnitOfWork) MarkRollback(ctx context.Context) {
	tenant.MarkRollback(ctx)
}

// WithTenantID scopes ctx to tenantID.
func (TenantUnitOfWork) WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

// TenantID returns the tenant of ctx, or zero.
func (TenantUnitOfWork) TenantID(ctx context.Context) int64 {
	return tenant.FromContext(ctx)
}
