//go:build testhooks

package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// FaultSite names the transaction-control step a StatementFaults pool fails.
type FaultSite string

const (
	FaultNone FaultSite = ""
	// FaultBegin fails BEGIN before it reaches the server.
	FaultBegin FaultSite = "BEGIN"
	// FaultSavepointRestore and FaultSavepointRelease run the real statement,
	// then report the injected failure.
	FaultSavepointRestore FaultSite = "ROLLBACK TO SAVEPOINT"
	FaultSavepointRelease FaultSite = "RELEASE SAVEPOINT"
	// FaultRollback runs the real ROLLBACK of the transaction, then reports
	// the injected failure.
	FaultRollback FaultSite = "ROLLBACK"
	// FaultCommitApplied commits for real, then reports the injected failure:
	// the rows are durable although COMMIT failed.
	FaultCommitApplied FaultSite = "COMMIT applied"
	// FaultCommitDiscarded rolls back instead of committing, then reports the
	// injected failure: nothing is durable.
	FaultCommitDiscarded FaultSite = "COMMIT discarded"
	// FaultProbeStatement fails ProbeStatement before it reaches the server,
	// as a driver reports a connection that broke under a statement.
	FaultProbeStatement FaultSite = "SELECT 'fault_probe'"
)

// ProbeStatement is the statement FaultProbeStatement fails.
const ProbeStatement = "SELECT 'fault_probe'"

// StatementFaults fails armed transaction-control steps of its pool, each
// once, in the order they occur (#6316). Every other statement runs on a real
// PostgreSQL connection.
type StatementFaults struct {
	mu    sync.Mutex
	armed []*armedFault
}

type armedFault struct {
	site   FaultSite
	prefix string
	err    error
	fired  bool
	// held keeps the statement until its context ends before it fails.
	held bool
}

// Arm replaces the armed failures with one: the next step at site fails
// with err, once.
func (f *StatementFaults) Arm(site FaultSite, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.armed = []*armedFault{{site: site, err: err}}
}

// AlsoFailStatement arms one more failure: the next statement starting with
// prefix fails with err before it reaches the server, once.
func (f *StatementFaults) AlsoFailStatement(prefix string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.armed = append(f.armed, &armedFault{site: faultStatementPrefix, prefix: prefix, err: err})
}

// AlsoFailStatementWhenCancelled arms one more failure: the next statement
// starting with prefix is held until its caller cancels its context, then
// fails with err instead of the cancellation, once. It is a statement that
// broke while its caller was stopping it.
func (f *StatementFaults) AlsoFailStatementWhenCancelled(prefix string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.armed = append(f.armed, &armedFault{site: faultStatementPrefix, prefix: prefix, err: err, held: true})
}

// AlsoFail arms one more failure at site.
func (f *StatementFaults) AlsoFail(site FaultSite, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.armed = append(f.armed, &armedFault{site: site, err: err})
}

// Fired reports whether every armed failure was reported.
func (f *StatementFaults) Fired() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, fault := range f.armed {
		if !fault.fired && fault.site != FaultNone {
			return false
		}
	}
	return len(f.armed) > 0
}

const faultStatementPrefix FaultSite = "statement"

func (f *StatementFaults) take(site FaultSite) error {
	_, err := f.takeStatement(site, "")
	return err
}

// statementFault is the failure armed for query, after its hold.
func (f *StatementFaults) statementFault(ctx context.Context, query string) error {
	held, err := f.takeStatement(faultStatementPrefix, strings.TrimSpace(query))
	if held {
		<-ctx.Done()
	}
	return err
}

func (f *StatementFaults) takeStatement(site FaultSite, query string) (held bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, fault := range f.armed {
		if fault.fired || fault.site != site {
			continue
		}
		if site == faultStatementPrefix && !strings.HasPrefix(query, fault.prefix) {
			continue
		}
		fault.fired = true
		return fault.held, fault.err
	}
	return false, nil
}

// NewStatementFaultPool opens a private pool on the isolated test database of
// tb whose connections fail the armed step. Its tenant runtime is
// TenantRuntime(tb, pool).
func NewStatementFaultPool(tb testing.TB) (*bun.DB, *StatementFaults) {
	tb.Helper()
	faults := &StatementFaults{}
	pool := AdoptPrivatePool(tb, NewBunDB(sql.OpenDB(faultingConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(IsolatedTestDSN(tb))), faults: faults,
	})))
	tb.Cleanup(func() { _ = pool.Close() })
	return pool, faults
}

// StatementFaultCells is one statement-fault pool on the isolated test
// database of tb for many cells, each with its own StatementFaults
// (NewStatementFaultCell): a pool and fresh connections per cell cost a
// PostgreSQL session start each.
type StatementFaultCells struct {
	cells *CellPool
}

// NewStatementFaultCells opens the shared pool on tb's isolated test database.
func NewStatementFaultCells(tb testing.TB) *StatementFaultCells {
	tb.Helper()
	return &StatementFaultCells{cells: NewCellPool(tb, faultingConnector{
		Connector: pgdriver.NewConnector(pgdriver.WithDSN(IsolatedTestDSN(tb))),
	})}
}

// NewStatementFaultCell returns the handle of one cell of cells and its
// faults: the steps the handle issues fail as armed on these faults, as on a
// NewStatementFaultPool pool. Its tenant runtime is TenantRuntime(tb, pool).
func NewStatementFaultCell(tb testing.TB, cells *StatementFaultCells) (*bun.DB, *StatementFaults) {
	tb.Helper()
	faults := &StatementFaults{}
	return cells.cells.Cell(tb, faults), faults
}

// InjectedFailure is a failure of kind: "" has no SQLSTATE, "bad_conn" is
// driver.ErrBadConn, anything else is a PostgreSQL error with that SQLSTATE.
func InjectedFailure(kind string) error {
	switch kind {
	case "":
		return errors.New("injected failure without SQLSTATE")
	case "bad_conn":
		return driver.ErrBadConn
	default:
		return pgdriver.NewError(map[byte]string{'S': "ERROR", 'V': "ERROR", 'C': kind, 'M': "injected " + kind})
	}
}

// RaiseSQLState ends a statement of tx with a real server error of SQLSTATE
// kind.
func RaiseSQLState(ctx context.Context, tx bun.Tx, kind string) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN RAISE EXCEPTION 'probe %s' USING ERRCODE = '%s'; END $$`, kind, kind))
	if err == nil {
		return errors.New("raise returned no error")
	}
	return err
}

// faultingConnector fails the armed steps of faults, or, without faults (a
// StatementFaultCells pool), of the cell whose handle issued each step.
type faultingConnector struct {
	driver.Connector
	faults *StatementFaults
}

func (c faultingConnector) Connect(ctx context.Context) (driver.Conn, error) {
	inner, faults := c.Connector, c.faults
	conn, err := inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &faultingConn{Conn: conn, faults: faults}, nil
}

type faultingConn struct {
	driver.Conn
	faults *StatementFaults // nil on a StatementFaultCells pool
}

// faultsOf are the faults that apply to the step of ctx: the pool's own, or
// the plan of the cell that issued it.
func (c *faultingConn) faultsOf(ctx context.Context) (*StatementFaults, error) {
	if c.faults != nil {
		return c.faults, nil
	}
	if faults, ok := CellPlan(ctx).(*StatementFaults); ok {
		return faults, nil
	}
	return nil, ErrStatementOutsideCell
}

func (c *faultingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	faults, err := c.faultsOf(ctx)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(strings.TrimSpace(query), ProbeStatement) {
		if fault := faults.take(FaultProbeStatement); fault != nil {
			return nil, fault
		}
	}
	if fault := faults.statementFault(ctx, query); fault != nil {
		return nil, fault
	}
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
	query = strings.TrimSpace(query)
	for _, site := range []FaultSite{FaultSavepointRestore, FaultSavepointRelease} {
		if strings.HasPrefix(query, string(site)) {
			if fault := faults.take(site); fault != nil {
				return nil, fault
			}
		}
	}
	return result, err
}

func (c *faultingConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	faults, err := c.faultsOf(ctx)
	if err != nil {
		return nil, err
	}
	if fault := faults.statementFault(ctx, query); fault != nil {
		return nil, fault
	}
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *faultingConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	faults, err := c.faultsOf(ctx)
	if err != nil {
		return nil, err
	}
	if fault := faults.take(FaultBegin); fault != nil {
		return nil, fault
	}
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return faultingTx{Tx: tx, faults: faults}, nil
}

func (c *faultingConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.Conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *faultingConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

func (c *faultingConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type faultingTx struct {
	driver.Tx
	faults *StatementFaults
}

func (t faultingTx) Commit() error {
	if fault := t.faults.take(FaultCommitApplied); fault != nil {
		if err := t.Tx.Commit(); err != nil {
			return err
		}
		return fault
	}
	if fault := t.faults.take(FaultCommitDiscarded); fault != nil {
		if err := t.Tx.Rollback(); err != nil {
			return err
		}
		return fault
	}
	return t.Tx.Commit()
}

func (t faultingTx) Rollback() error {
	err := t.Tx.Rollback()
	if fault := t.faults.take(FaultRollback); fault != nil {
		return fault
	}
	return err
}

// FaultProbe is a statement-fault pool with a probe table both tenant and
// admin units may write, plus the superuser pool that inspects it.
type FaultProbe struct {
	Super  *bun.DB
	Pool   *bun.DB
	Faults *StatementFaults
}

// NewFaultProbe creates the probe table public.fault_probe in tb's isolated
// test database and opens a StatementFaults pool on it.
func NewFaultProbe(tb testing.TB) *FaultProbe {
	tb.Helper()
	super := SetupIsolatedTestDB(tb)
	if _, err := super.ExecContext(context.Background(), `CREATE TABLE public.fault_probe (attempt int NOT NULL);
GRANT SELECT, INSERT ON public.fault_probe TO phoenix_tenant, phoenix_admin`); err != nil {
		tb.Fatalf("create fault probe table: %v", err)
	}
	pool, faults := NewStatementFaultPool(tb)
	return &FaultProbe{Super: super, Pool: pool, Faults: faults}
}

// Write inserts one probe row in tx.
func (p *FaultProbe) Write(ctx context.Context, tx bun.Tx, attempt int) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO public.fault_probe (attempt) VALUES (?)`, attempt)
	return err
}

// Reset empties the probe table.
func (p *FaultProbe) Reset(tb testing.TB) {
	tb.Helper()
	super := p.Super
	if _, err := super.ExecContext(context.Background(), `TRUNCATE public.fault_probe`); err != nil {
		tb.Fatalf("reset fault probe: %v", err)
	}
}

// Rows counts the committed probe rows.
func (p *FaultProbe) Rows(tb testing.TB) int {
	tb.Helper()
	var rows int
	super := p.Super
	if err := super.NewRaw(`SELECT count(*) FROM public.fault_probe`).Scan(context.Background(), &rows); err != nil {
		tb.Fatalf("count fault probe rows: %v", err)
	}
	return rows
}

// TerminateBackend ends tx's backend from the superuser pool, waits for it
// and returns the error of the next statement on the dead connection.
func (p *FaultProbe) TerminateBackend(ctx context.Context, tx bun.Tx) error {
	var pid int
	if err := tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
		return err
	}
	var terminated bool
	if err := p.Super.NewRaw(`SELECT pg_terminate_backend(?, 5000)`, pid).Scan(ctx, &terminated); err != nil {
		return err
	}
	if !terminated {
		return errors.New("backend was not terminated")
	}
	if _, err := tx.ExecContext(ctx, `SELECT 1`); err != nil {
		return err
	}
	return errors.New("statement on a terminated backend succeeded")
}
