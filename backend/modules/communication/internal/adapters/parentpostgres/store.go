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
type TenantScope struct {
	TenantID      int64
	AcrossSchools bool
}

// ScopeFor derives cross-school access only from an actual administrative
// transaction. Missing school context alone never grants it.
func ScopeFor(schoolID int64, admin bool) TenantScope {
	return TenantScope{TenantID: schoolID, AcrossSchools: admin && schoolID == 0}
}
func (s TenantScope) Validate() error {
	if s.AcrossSchools {
		return nil
	}
	if s.TenantID <= 0 {
		return errors.New("communication: tenant is required")
	}
	return nil
}

type Database func(context.Context) (bun.IDB, TenantScope, error)

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
}](query Q, alias string, scope TenantScope) Q {
	if err := scope.Validate(); err != nil {
		return query.Err(err)
	}
	if scope.AcrossSchools {
		return query
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, scope.TenantID)
}

// ResolveDatabase validates the selected school/admin scope before resolving
// the SQL handle. Unknown transaction values are never replaced with a pool.
func ResolveDatabase(raw any, present bool, pool *bun.DB, scope TenantScope) (bun.IDB, TenantScope, error) {
	if err := scope.Validate(); err != nil {
		return nil, scope, err
	}
	if !present {
		return pool, scope, nil
	}
	switch tx := raw.(type) {
	case bun.Tx:
		return tx, scope, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, scope, nil
		}
	}
	return nil, scope, fmt.Errorf("communication: unsupported transaction %T", raw)
}
