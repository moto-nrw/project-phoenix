package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
)

func retainedNewDetachedContext() func(ctx context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		return tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx))
	}
}
