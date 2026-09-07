// Package parentpostgres holds Communication's persistence for the
// tenant-scoped parent surfaces: guardian notification consent, parent/OGS
// message threads, and parent announcements with their replies.
//
// It is the only place that issues SQL against users.notification_preferences,
// users.parent_message*, and users.parent_announcement* — the tables
// Communication owns. Callers reach it through the module facade, never
// directly.
package parentpostgres

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient transaction plus the tenant the
// statement must be scoped to. The composition root supplies it, so this
// package stays free of transaction- and tenant-runtime details.
//
// A zero tenant means "no tenant in this context" (administrative or
// cross-tenant work); statements then rely on RLS alone, exactly as the
// repositories they replace did.
type Database func(context.Context) (bun.IDB, int64, error)

// ErrDatabaseRequired reports a store constructed without a database runtime.
var ErrDatabaseRequired = errors.New("communication parent store: database runtime is required")

type store struct{ database Database }

func newStore(database Database) store {
	if database == nil {
		panic(ErrDatabaseRequired.Error())
	}
	return store{database: database}
}

// withTenant applies the defense-in-depth tenant_id filter that complements
// RLS. A zero tenant leaves the query untouched, matching the behaviour of the
// repositories this package replaces.
func withTenant[Q interface{ Where(string, ...any) Q }](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
}
