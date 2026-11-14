package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/uptrace/bun"
)

// CellPool is one database/sql pool on a fault-injecting connector whose
// connections serve the cells of a fault matrix in turn, instead of a pool
// and fresh connections per cell. A cell reaches it only through its own
// handle (Cell), and the connector reads the cell's fault plan from each
// statement's context (CellPlan), never from the pool: a plan cannot reach
// the statements of another cell, whichever connection they share.
//
// Connections return to the pool as healthy sessions only: a statement the
// connector fails never reached the server or ran for real before the
// failure, transactions end with a real ROLLBACK or COMMIT, and the tenant
// runtime sets role and tenant with set_config(..., true), local to the
// transaction. A connection a cell breaks (driver.ErrBadConn, a closed
// pgdriver connection) is discarded by database/sql as on a private pool.
type CellPool struct {
	db *sql.DB
}

// NewCellPool opens connector as one pool for tb and its subtests; it closes
// with tb. Like the package pool it keeps up to poolSize() idle connections,
// so the cells running at the same time reuse them; open connections stay
// unbounded, as on the private pools it replaces.
func NewCellPool(tb testing.TB, connector driver.Connector) *CellPool {
	tb.Helper()
	db := sql.OpenDB(connector)
	db.SetMaxIdleConns(poolSize())
	db.SetConnMaxIdleTime(10 * time.Minute)
	tb.Cleanup(func() { _ = db.Close() })
	return &CellPool{db: db}
}

// Cell returns the handle of one cell: every statement, BEGIN, COMMIT and
// ROLLBACK issued through it, on the handle, a transaction or a connection
// of it, carries plan in its context. Query counters and lock-wait evidence
// attach to the handle and see this cell's statements only. Never close the
// handle: it shares the pool of every cell.
func (p *CellPool) Cell(tb testing.TB, plan any) *bun.DB {
	tb.Helper()
	if plan == nil {
		tb.Fatal("CellPool.Cell needs a plan")
	}
	return AdoptPrivatePool(tb, NewBunDB(p.db).WithQueryHook(cellPlanHook{plan: plan}))
}

// ErrStatementOutsideCell fails a statement that reached a CellPool without
// a cell's plan: it bypassed the cell handles, so no plan may apply to it.
var ErrStatementOutsideCell = errors.New("statement reached the cell pool without a cell plan")

// CellPlan is the plan of the cell whose handle issued the statement of ctx,
// or nil outside a cell.
func CellPlan(ctx context.Context) any {
	return ctx.Value(cellPlanKey{})
}

type cellPlanKey struct{}

// cellPlanHook puts the cell's plan into the context bun passes on to the
// driver for each statement, BEGIN, COMMIT and ROLLBACK of the handle.
type cellPlanHook struct{ plan any }

func (h cellPlanHook) BeforeQuery(ctx context.Context, _ *bun.QueryEvent) context.Context {
	return context.WithValue(ctx, cellPlanKey{}, h.plan)
}

func (cellPlanHook) AfterQuery(context.Context, *bun.QueryEvent) {}
