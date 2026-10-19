package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"

	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// Runtime is the tenant transaction runtime the retained People Directory
// repositories run on. The composition root binds it to the shared tenant
// runtime (peopledirectory/compose.LegacyRepositoryRuntime), so this package
// reads neither the tenant context nor the transaction protocol itself
// (#2727).
type Runtime interface {
	// DB is the ambient tenant transaction, or the pool outside one.
	DB(ctx context.Context) (bun.IDB, error)
	// TenantID is the school the caller acts for, 0 when none.
	TenantID(ctx context.Context) int64
	// RequireTenantID is TenantID, failing when the context carries none.
	RequireTenantID(ctx context.Context) (int64, error)
	// AcrossSchools is true only inside an explicitly administrative transaction without a school scope.
	AcrossSchools(ctx context.Context) bool
	// InTransaction reports whether ctx carries a tenant transaction.
	InTransaction(ctx context.Context) bool
	// WithSavepoint runs fn behind a savepoint of the ambient transaction.
	WithSavepoint(ctx context.Context, fn func(context.Context) error) error
}

func requireRuntime(runtime Runtime) Runtime {
	if runtime == nil {
		panic("people directory repository: runtime is required")
	}
	return runtime
}

// withTenantFilter adds the defense-in-depth tenant_id filter for alias when
// the context carries a tenant, on top of PostgreSQL RLS.
func withTenantFilter[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](ctx context.Context, runtime Runtime, q Q, alias string) Q {
	tenantID, err := runtime.RequireTenantID(ctx)
	if err != nil {
		if runtime.AcrossSchools(ctx) {
			return q
		}
		return q.Err(err)
	}
	return q.Where(fmt.Sprintf(`"%s".tenant_id = ?`, alias), tenantID)
}

// translateNotFound joins the repository not-found sentinel onto a missing
// row, so callers classify with errors.Is(err, sql.ErrNoRows) and the
// RepositoryNotFound marker alike.
func translateNotFound(err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.Join(ports.ErrRepositoryNotFound, err)
}

// assertRowsAffected checks that a DML statement affected exactly the
// expected number of rows.
func assertRowsAffected(result sql.Result, expected int64, op string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	return assertRowsAffectedCount(n, expected, op)
}

func assertRowsAffectedCount(actual, expected int64, op string) error {
	if actual != expected {
		return &ports.DatabaseError{
			Op:  op,
			Err: fmt.Errorf("expected %d rows affected, got %d", expected, actual),
		}
	}
	return nil
}

type tenantScopedEntity interface {
	GetTenantID() int64
	SetTenantID(int64)
}

// ensureTenantID sets a tenant-scoped entity's tenant from the context when
// it has none yet.
func ensureTenantID(ctx context.Context, runtime Runtime, entity any) {
	if scoped, ok := entity.(tenantScopedEntity); ok && scoped.GetTenantID() == 0 {
		if id := runtime.TenantID(ctx); id != 0 {
			scoped.SetTenantID(id)
		}
	}
}

// inExistingTransaction never starts a unit of work. The application command
// or workflow owns its commit and rollback boundary.
func inExistingTransaction(ctx context.Context, runtime Runtime, fn func(context.Context) error) error {
	if !runtime.InTransaction(ctx) {
		return errors.New("people directory repository: transaction is required")
	}
	return fn(ctx)
}
