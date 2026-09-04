package calendar

import (
	"context"

	"github.com/uptrace/bun"
)

// Runtime is the composition-owned bridge to the request transaction and
// tenant. Calendar persistence does not know how those values are propagated.
type Runtime struct {
	Database func(context.Context) bun.IDB
	TenantID func(context.Context) int64
}

func (runtime Runtime) validate() {
	if runtime.Database == nil || runtime.TenantID == nil {
		panic("calendar repository: runtime is required")
	}
}

type tenantQuery[Q any] interface {
	Where(string, ...any) Q
}

func withTenantFilter[Q tenantQuery[Q]](runtime Runtime, ctx context.Context, query Q, alias string) Q {
	if tenantID := runtime.TenantID(ctx); tenantID > 0 {
		return query.Where("? = ?", bun.Ident(alias+".tenant_id"), tenantID)
	}
	return query
}
