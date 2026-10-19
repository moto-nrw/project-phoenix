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
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// Database resolves the caller connection and school; school-specific queries
// reject an absent school instead of removing the tenant predicate.
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

// assertOneRow turns a write that matched nothing into an explicit error. A
// silent no-op would let a caller believe a row it never touched was updated.
func assertOneRow(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	if affected != 1 {
		return fmt.Errorf("%s: expected 1 row, affected %d", operation, affected)
	}
	return nil
}

// withTenant requires a school and adds its predicate on top of RLS.
func withTenant[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query.Err(errors.New("communication: tenant is required"))
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
}
