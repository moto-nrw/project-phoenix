//go:build testhooks

package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/uptrace/bun/driver/pgdriver"
)

// BeginFault makes the next BEGIN of its pool fail once with a broken
// connection, as a dropped connection does, and counts every BEGIN the pool
// attempts. Tests use it to prove that a unit of work neither retries nor
// swallows a failed start.
//
// It also injects the other failures of a unit's own frame, each once and
// each counted from the moment it is armed: the n-th BEGIN, the n-th
// statement starting with a prefix (a role switch, say), the n-th COMMIT or
// ROLLBACK. AfterStatement runs a callback once after the rows of the first
// statement with a prefix are closed, for example to cancel the request.
type BeginFault struct {
	armed  atomic.Bool
	begins atomic.Int64

	mu     sync.Mutex
	faults []*poolFault
}

type faultKind int

const (
	faultBegin faultKind = iota
	faultStatement
	faultCommit
	faultRollback
	faultAfterStatement
)

// poolFault is one armed injection: skip matching events pass, the next one
// fails (or runs after).
type poolFault struct {
	kind   faultKind
	prefix string
	skip   int
	after  func()
}

// Arm makes the next BEGIN fail.
func (f *BeginFault) Arm() { f.armed.Store(true) }

// Begins is the number of BEGINs the pool attempted, failed ones included.
func (f *BeginFault) Begins() int64 { return f.begins.Load() }

// ArmBegin makes the BEGIN after skip further BEGINs fail once.
func (f *BeginFault) ArmBegin(skip int) { f.add(&poolFault{kind: faultBegin, skip: skip}) }

// ArmStatement makes the statement starting with prefix, after skip such
// statements, fail once.
func (f *BeginFault) ArmStatement(prefix string, skip int) {
	f.add(&poolFault{kind: faultStatement, prefix: prefix, skip: skip})
}

// ArmCommit makes the COMMIT after skip further COMMITs fail once.
func (f *BeginFault) ArmCommit(skip int) { f.add(&poolFault{kind: faultCommit, skip: skip}) }

// ArmRollback makes the ROLLBACK after skip further ROLLBACKs fail once. The
// transaction stays open on the server; the connection is discarded.
func (f *BeginFault) ArmRollback(skip int) { f.add(&poolFault{kind: faultRollback, skip: skip}) }

// AfterStatement runs fn once, after the rows of the first statement starting
// with prefix are closed.
func (f *BeginFault) AfterStatement(prefix string, fn func()) {
	f.add(&poolFault{kind: faultAfterStatement, prefix: prefix, after: fn})
}

// Pending reports whether an armed injection has not fired yet.
func (f *BeginFault) Pending() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.faults) > 0 || f.armed.Load()
}

// Reset drops every injection that has not fired.
func (f *BeginFault) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.faults = nil
	f.armed.Store(false)
}

func (f *BeginFault) add(fault *poolFault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.faults = append(f.faults, fault)
}

// fire reports whether the event of kind (with statement query) hits an armed
// injection, and returns it after removing it.
func (f *BeginFault) fire(kind faultKind, query string) *poolFault {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, fault := range f.faults {
		if fault.kind != kind || (fault.prefix != "" && !strings.HasPrefix(strings.TrimSpace(query), fault.prefix)) {
			continue
		}
		if fault.skip > 0 {
			fault.skip--
			continue
		}
		f.faults = append(f.faults[:i], f.faults[i+1:]...)
		return fault
	}
	return nil
}

// OpenPostgresSQLWithBeginFault opens a pool on dsn whose connections fail a
// BEGIN once when fault is armed. Every other driver call reaches pgdriver
// unchanged unless an injection is armed for it.
func OpenPostgresSQLWithBeginFault(dsn string) (*sql.DB, *BeginFault) {
	fault := &BeginFault{}
	return sql.OpenDB(beginFaultConnector{inner: pgdriver.NewConnector(pgdriver.WithDSN(dsn)), fault: fault}), fault
}

type beginFaultConnector struct {
	inner driver.Connector
	fault *BeginFault
}

func (c beginFaultConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &beginFaultConn{Conn: conn.(*pgdriver.Conn), fault: c.fault}, nil
}

func (c beginFaultConnector) Driver() driver.Driver { return c.inner.Driver() }

// beginFaultConn embeds the pgdriver connection, so database/sql sees every
// optional interface it implements; BeginTx and statement execution are
// intercepted.
type beginFaultConn struct {
	*pgdriver.Conn
	fault *BeginFault
}

func (c *beginFaultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.fault.begins.Add(1)
	if c.fault.armed.CompareAndSwap(true, false) || c.fault.fire(faultBegin, "") != nil {
		return nil, driver.ErrBadConn
	}
	tx, err := c.Conn.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return faultTx{Tx: tx, fault: c.fault}, nil
}

func (c *beginFaultConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.fault.fire(faultStatement, query) != nil {
		return nil, driver.ErrBadConn
	}
	result, err := c.Conn.ExecContext(ctx, query, args)
	if after := c.fault.fire(faultAfterStatement, query); after != nil {
		after.after()
	}
	return result, err
}

func (c *beginFaultConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.fault.fire(faultStatement, query) != nil {
		return nil, driver.ErrBadConn
	}
	rows, err := c.Conn.QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}
	if after := c.fault.fire(faultAfterStatement, query); after != nil {
		return &afterCloseRows{Rows: rows, after: after.after}, nil
	}
	return rows, nil
}

// afterCloseRows runs after once its rows are closed.
type afterCloseRows struct {
	driver.Rows
	after func()
	once  sync.Once
}

func (r *afterCloseRows) Close() error {
	err := r.Rows.Close()
	r.once.Do(r.after)
	return err
}

type faultTx struct {
	driver.Tx
	fault *BeginFault
}

func (t faultTx) Commit() error {
	if t.fault.fire(faultCommit, "") != nil {
		return driver.ErrBadConn
	}
	return t.Tx.Commit()
}

func (t faultTx) Rollback() error {
	if t.fault.fire(faultRollback, "") != nil {
		return driver.ErrBadConn
	}
	return t.Tx.Rollback()
}
