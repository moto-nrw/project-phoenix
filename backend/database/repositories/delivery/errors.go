package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// DatabaseError is a failed push subscription statement: the store's failure,
// not a refusal of the request.
type DatabaseError struct {
	Op  string // statement that failed
	Err error  // driver error
}

func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *DatabaseError) Unwrap() error { return e.Err }

// StoreFailure marks the error as the store's failure for callers that may
// not name this type.
func (e *DatabaseError) StoreFailure() bool { return true }

// errNotFound is the repository not-found result. Callers recognise it by its
// RepositoryNotFound marker, the shape every retained repository shares.
var errNotFound error = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string       { return "repository: not found" }
func (notFoundError) RepositoryNotFound() {}

// translateNotFound joins the not-found marker onto a missing row and keeps
// sql.ErrNoRows in the chain.
func translateNotFound(err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.Join(errNotFound, err)
}

// conn is the ambient tenant transaction, or the pool outside one.
func (r *PushSubscriptionRepository) conn(ctx context.Context) bun.IDB {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return r.db
	}
	switch tx := transaction.(type) {
	case bun.Tx:
		return tx
	case *bun.Tx:
		if tx != nil {
			return tx
		}
	}
	panic(fmt.Sprintf("push subscription repository: unsupported transaction type %T", transaction))
}

// withTenantFilter adds the defense-in-depth tenant_id filter for alias when
// the context carries a tenant, on top of PostgreSQL RLS.
func withTenantFilter[Q interface{ Where(string, ...any) Q }](ctx context.Context, q Q, alias string) Q {
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		return q.Where(fmt.Sprintf(`"%s".tenant_id = ?`, alias), tenantID)
	}
	return q
}
