package database

import (
	"context"
	"database/sql/driver"
	"net"

	"github.com/uptrace/bun/driver/pgdriver"
)

// discardClosedConn runs before database/sql hands out a pooled connection.
// When Postgres shuts down or restarts, it ends each idle session with a FATAL
// message (SQLSTATE 57P01) and closes the socket. pgdriver only notices on the
// next query and returns that FATAL error to the caller, so database/sql never
// retries on a fresh connection. An idle connection has nothing to read; a
// pending byte or EOF means the server ended the session.
// driver.ErrBadConn makes database/sql close it and take another connection.
func discardClosedConn(_ context.Context, cn *pgdriver.Conn) error {
	if !connAlive(cn.Conn()) {
		return driver.ErrBadConn
	}
	return nil
}

// connAlive peeks at the socket without blocking. Connections it cannot
// inspect count as alive and keep the previous behaviour.
func connAlive(conn net.Conn) bool {
	// A TLS connection exposes its socket through NetConn.
	if wrapped, ok := conn.(interface{ NetConn() net.Conn }); ok {
		conn = wrapped.NetConn()
	}
	return socketIdle(conn)
}
