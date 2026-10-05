package education

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/uptrace/bun"
)

// Runtime is the tenant transaction runtime the retained School Structure
// repositories run on. The composition binds it to the shared tenant runtime
// (schoolstructure/compose.LegacyRepositoryRuntime), so this package reads
// neither the tenant context nor the transaction protocol itself (#2742).
type Runtime interface {
	// DB is the ambient tenant transaction, or the pool outside one.
	DB(ctx context.Context) bun.IDB
	// TenantID is the school the caller acts for, 0 when none.
	TenantID(ctx context.Context) int64
}

func requireRuntime(runtime Runtime) Runtime {
	if runtime == nil {
		panic("school structure repository: runtime is required")
	}
	return runtime
}

// withTenantFilter adds the defense-in-depth tenant_id filter for alias when
// the context carries a school, on top of PostgreSQL RLS.
func withTenantFilter[Q interface{ Where(string, ...any) Q }](ctx context.Context, runtime Runtime, q Q, alias string) Q {
	if tenantID := runtime.TenantID(ctx); tenantID > 0 {
		return q.Where(fmt.Sprintf(`"%s".tenant_id = ?`, alias), tenantID)
	}
	return q
}

// translateNotFound joins the School Structure not-found sentinel onto a
// missing row, so callers classify with errors.Is(err, sql.ErrNoRows) and the
// RepositoryNotFound marker alike.
func translateNotFound(err error) error {
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return errors.Join(education.ErrNotFound, err)
}

// assertRowsAffected checks that a DML statement affected exactly the
// expected number of rows.
func assertRowsAffected(result sql.Result, expected int64, op string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if n != expected {
		return &education.DatabaseError{
			Op:  op,
			Err: fmt.Errorf("expected %d rows affected, got %d", expected, n),
		}
	}
	return nil
}
