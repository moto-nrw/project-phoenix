package groups

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// RepositoryRuntime provides the context-only tenant capabilities of the owner.
type RepositoryRuntime struct{}

// TenantID is the school the caller acts for, 0 when none.
func (RepositoryRuntime) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// RunInTx joins the ambient tenant transaction or opens one for the
// context's school. The retained School Structure services run their writes
// through it.
func (RepositoryRuntime) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, fn)
}

// MarkRollback asks the request's tenant transaction to roll back.
func (RepositoryRuntime) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }
