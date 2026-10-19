package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

func NewContextDatabase[ID interface{ Int64() int64 }](school func(context.Context) (ID, error), transaction func(context.Context) (any, bool), admin func(context.Context) bool) Database {
	return func(ctx context.Context) (bun.IDB, TenantScope, error) {
		id, scopeErr := school(ctx)
		raw, ok := transaction(ctx)
		return ResolveDatabase(raw, ok, id.Int64(), scopeErr, admin(ctx))
	}
}
