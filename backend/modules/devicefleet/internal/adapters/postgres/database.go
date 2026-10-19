package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

// Database resolves an existing transaction and an explicit school or
// administrative cross-school scope. Device-key bootstrap selects that scope
// in the application before reading records.
type Database func(context.Context) (bun.IDB, TenantScope, error)
