package postgres

import (
	"context"

	"github.com/uptrace/bun"
)

// Database resolves the connection or ambient transaction plus the caller's
// tenant. A zero tenant means "no tenant predicate", which the device-auth
// path relies on when it resolves an API key before any tenant is known.
type Database func(context.Context) (bun.IDB, int64, error)
