package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func DetachedContext(ctx context.Context) context.Context {
	return context.WithoutCancel(tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx)))
}
