package tenant

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant/internal/testseam"
)

// The registration below is the only production link to tenant/tenanttest. It
// exports nothing: testseam is importable only inside the tenant subtree.
func init() {
	testseam.WithTransaction = withTransaction
	testseam.WithAfterCommitHooks = withTestAfterCommitHooks
}

func withTestAfterCommitHooks(ctx context.Context) (context.Context, func()) {
	newCtx, h := withAfterCommitHooks(ctx)
	return newCtx, func() { runAfterCommitHooks(h) }
}
