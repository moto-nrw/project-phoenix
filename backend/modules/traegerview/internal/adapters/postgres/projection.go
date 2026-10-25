// Package postgres implements the Träger projection's reads (ADR 0084
// section 1). Every statement is a compile-time constant so the architecture
// evaluator can read which tables it touches, and every statement filters on
// the school set of the Träger through its first placeholder, `= ANY(?0)`,
// bound to that set: backend/test/traeger_view_scope_ratchet_test.go rejects
// any query of this package that does not, and any other query builder.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// Database resolves the caller's ambient administrative transaction.
type Database func(context.Context) (bun.IDB, error)

// Projection reads across the schools of one Träger.
type Projection struct {
	database Database
}

// New builds the projection over the ambient transaction runtime.
func New(database Database) *Projection {
	if database == nil {
		panic("traeger view postgres: database runtime is required")
	}
	return &Projection{database: database}
}

// AmbientAdminDB resolves the projection's administrative transaction. The
// projection never reads in a tenant transaction or without one.
func AmbientAdminDB(ctx context.Context) (bun.IDB, error) {
	value, ok := tenant.TransactionFromContext(ctx)
	if !ok || !tenant.IsAdminTx(ctx) {
		return nil, errors.New("traeger view postgres: the administrative transaction is required")
	}
	switch tx := value.(type) {
	case bun.Tx:
		return tx, nil
	case *bun.Tx:
		if tx != nil {
			return *tx, nil
		}
	}
	return nil, fmt.Errorf("traeger view postgres: unsupported transaction %T", value)
}

var errNoSchools = errors.New("traeger view postgres: a scoped query needs the school set of the Träger")

// scopedDB returns the transaction for one query over the school set. An
// empty set reads nothing.
func (p *Projection) scopedDB(ctx context.Context, schools []int64) (bun.IDB, error) {
	if len(schools) == 0 {
		return nil, errNoSchools
	}
	return p.database(ctx)
}

// schoolsQuery lists the schools of the set. platform.schools is keyed by the
// school ID, which is the tenant ID.
const schoolsQuery = `SELECT "school".id, "school".name, "school".slug, "school".subdomain,
		COALESCE("school".city, '') AS city
	FROM platform.schools AS "school"
	WHERE "school".id = ANY(?0) AND "school".deleted_at IS NULL
	ORDER BY "school".name, "school".id`

// Schools lists the schools of the set, ordered by name.
func (p *Projection) Schools(ctx context.Context, schools []int64) ([]traegerview.School, error) {
	result := []traegerview.School{}
	if len(schools) == 0 {
		return result, nil
	}
	db, err := p.scopedDB(ctx, schools)
	if err != nil {
		return nil, err
	}
	if err := db.NewRaw(schoolsQuery, pgdialect.Array(schools)).Scan(ctx, &result); err != nil {
		return nil, fmt.Errorf("traeger view postgres: list schools: %w", err)
	}
	return result, nil
}
