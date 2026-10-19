package audit

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// Runtime resolves the caller's authoritative transaction and tenant. Audit
// repositories never start a transaction and never fall back to the root DB.
type Runtime func(context.Context) (bun.IDB, int64)

func requireRuntime(runtime Runtime) Runtime {
	if runtime == nil {
		panic("audit repository: runtime is required")
	}
	return runtime
}

func database(ctx context.Context, runtime Runtime) (bun.IDB, int64, error) {
	db, tenantID := runtime(ctx)
	if db == nil {
		return nil, 0, errors.New("audit repository: transaction is required")
	}
	return db, tenantID, nil
}

func runtimeDB(ctx context.Context, runtime Runtime) (bun.IDB, error) {
	db, _, err := database(ctx, requireRuntime(runtime))
	return db, err
}

func runtimeTenantID(ctx context.Context, runtime Runtime) int64 {
	_, tenantID := requireRuntime(runtime)(ctx)
	return tenantID
}

func prepareTenant(ctx context.Context, runtime Runtime, event interface {
	GetTenantID() int64
	SetTenantID(int64)
}) (bun.IDB, error) {
	db, tenantID, err := database(ctx, runtime)
	if err != nil {
		return nil, err
	}
	if tenantID <= 0 {
		if event.GetTenantID() <= 0 {
			return nil, errors.New("audit repository: tenant is required")
		}
		return db, nil
	}
	if event.GetTenantID() == 0 {
		event.SetTenantID(tenantID)
	}
	if event.GetTenantID() != tenantID {
		return nil, fmt.Errorf("audit repository: event tenant %d does not match transaction tenant %d", event.GetTenantID(), tenantID)
	}
	return db, nil
}

// DatabaseError identifies failures in the Audit persistence adapter without
// importing the transaction-runtime domain package.
type DatabaseError struct {
	Op  string
	Err error
}

func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "audit database error during " + e.Op
	}
	return "audit database error during " + e.Op + ": " + e.Err.Error()
}

func (e *DatabaseError) Unwrap() error { return e.Err }

func wrapDatabase(op string, err error) error {
	if err == nil {
		return nil
	}
	return &DatabaseError{Op: op, Err: err}
}

// tenantQuery rejects an absent school before building an owner-scoped read.
func tenantQuery(ctx context.Context, runtime Runtime, query *bun.SelectQuery, alias string) *bun.SelectQuery {
	id := runtimeTenantID(ctx, runtime)
	if id <= 0 {
		return query.Err(errors.New("audit repository: tenant is required"))
	}
	return query.Where("?.tenant_id = ?", bun.Ident(alias), id)
}
