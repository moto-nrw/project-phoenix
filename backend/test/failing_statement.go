package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// FailInCurrentTransaction runs a statement PostgreSQL refuses in the
// transaction of ctx. Such an error aborts the whole transaction unless a
// savepoint contains it, which lets a test prove that a best-effort step
// cannot take the rest of its unit of work down with it.
func FailInCurrentTransaction(ctx context.Context) error {
	tx, err := tenant.CurrentTransaction[bun.IDB](ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "SELECT 1 / 0")
	return err
}

// PingCurrentTransaction runs a harmless statement in the transaction of ctx.
// It fails once an earlier error aborted that transaction, so a test can see
// whether a failed step was contained by its savepoint.
func PingCurrentTransaction(ctx context.Context) error {
	tx, err := tenant.CurrentTransaction[bun.IDB](ctx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "SELECT 1")
	return err
}
