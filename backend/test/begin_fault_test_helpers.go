//go:build testhooks

package test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync/atomic"

	"github.com/uptrace/bun/driver/pgdriver"
)

// BeginFault makes the next BEGIN of its pool fail once with a broken
// connection, as a dropped connection does, and counts every BEGIN the pool
// attempts. Tests use it to prove that a unit of work neither retries nor
// swallows a failed start.
type BeginFault struct {
	armed  atomic.Bool
	begins atomic.Int64
}

// Arm makes the next BEGIN fail.
func (f *BeginFault) Arm() { f.armed.Store(true) }

// Begins is the number of BEGINs the pool attempted, failed ones included.
func (f *BeginFault) Begins() int64 { return f.begins.Load() }

// OpenPostgresSQLWithBeginFault opens a pool on dsn whose connections fail a
// BEGIN once when fault is armed. Every other driver call reaches pgdriver
// unchanged.
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
// optional interface it implements; only BeginTx is intercepted.
type beginFaultConn struct {
	*pgdriver.Conn
	fault *BeginFault
}

func (c *beginFaultConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.fault.begins.Add(1)
	if c.fault.armed.CompareAndSwap(true, false) {
		return nil, driver.ErrBadConn
	}
	return c.Conn.BeginTx(ctx, opts)
}
