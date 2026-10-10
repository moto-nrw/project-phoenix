package enrollmenthttp

import (
	"context"
	"net/http"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// The routes open their transactions through the shared tenant runtime the
// request carries; none of them holds a database. A step that runs inside a
// route already under a tenant transaction joins it, so an error returned
// from fn rolls the whole request back.

// withinTenant runs fn in the transaction of schoolID.
func withinTenant(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(schoolID)
	if err != nil {
		tenant.ObserveMissingTenant(ctx, err)
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}

// withinAdmin runs fn in the cross-tenant administrative transaction, for
// the school lookups behind the public links.
func withinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return tenant.WithinAdmin(ctx, fn)
}

// runInTenantTx wraps the request's tenant context in a tenant
// transaction so the service's repo calls hit the right RLS scope.
// A bare Resource from a route unit test runs fn on the request context.
func (rs *Resource) runInTenantTx(r *http.Request, fn func(ctx context.Context) error) error {
	if rs.runInTenantTxForTest != nil {
		return rs.runInTenantTxForTest(r, fn)
	}
	ctx := r.Context()
	if !rs.transactions {
		return fn(ctx)
	}
	return withinTenant(ctx, tenant.FromContext(ctx), fn)
}
