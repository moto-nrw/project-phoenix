// Package staffpostgres holds Communication's persistence for OGS-internal
// colleague conversations (#2598): threads, their participants, the message
// log and each reader's cursor.
//
// It is the only place that writes users.staff_message_threads,
// users.staff_message_participants, users.staff_messages and
// users.staff_message_reads — the tables Communication owns (#3221). The
// inbox and badge reads live in the staffinbox projection. Every statement
// names the users schema explicitly: the least-privilege phoenix_tenant role
// cannot resolve an unqualified name because its search_path excludes that
// schema.
package staffpostgres

import (
	"context"
	"errors"

	"github.com/uptrace/bun"
)

// Database resolves the caller connection and school; school-specific queries
// reject an absent school instead of removing the tenant predicate.
type Database func(context.Context) (bun.IDB, int64, error)

// ErrDatabaseRequired reports a store constructed without a database runtime.
var ErrDatabaseRequired = errors.New("communication staff store: database runtime is required")

type store struct{ database Database }

func newStore(database Database) store {
	if database == nil {
		panic(ErrDatabaseRequired.Error())
	}
	return store{database: database}
}

// withTenant requires a school and adds its predicate on top of RLS.
func withTenant[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](query Q, alias string, tenantID int64) Q {
	if tenantID <= 0 {
		return query.Err(errors.New("tenant is required"))
	}
	return query.Where(`"`+alias+`".tenant_id = ?`, tenantID)
}
