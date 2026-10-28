package database

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sentry BACKEND-13: after a Postgres restart every pooled connection held a
// FATAL 57P01 message and EOF, and each scheduler job failed on one of them.
func TestConnAliveDetectsSessionsTheServerEnded(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		server func(net.Conn)
		alive  bool
	}{
		{name: "idle", server: func(net.Conn) {}, alive: true},
		{name: "pending message", server: func(c net.Conn) { _, _ = c.Write([]byte("E")) }, alive: false},
		{name: "closed", server: func(c net.Conn) { _ = c.Close() }, alive: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client, server := tcpPair(t)
			test.server(server)

			// The peer's bytes and FIN reach the loopback socket asynchronously.
			if test.alive {
				time.Sleep(20 * time.Millisecond)
				assert.True(t, connAlive(client))
				return
			}
			assert.Eventually(t, func() bool { return !connAlive(client) }, time.Second, 5*time.Millisecond)
		})
	}
}

func TestConnAliveLeavesPendingBytesUnread(t *testing.T) {
	t.Parallel()
	client, server := tcpPair(t)
	_, err := server.Write([]byte("E"))
	require.NoError(t, err)

	require.Eventually(t, func() bool { return !connAlive(client) }, time.Second, 5*time.Millisecond)
	buf := make([]byte, 1)
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = client.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "E", string(buf), "the check peeks and does not consume")
}

func TestNewConnectorDiscardsClosedPoolConnections(t *testing.T) {
	t.Parallel()
	connector, err := newConnector("postgres://postgres:secret-value@localhost:5432/postgres?sslmode=disable")
	require.NoError(t, err)
	assert.NotNil(t, connector.Config().ResetSessionFunc, "pooled connections are checked before reuse")
}

func tcpPair(t *testing.T) (client, server net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan net.Conn, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr == nil {
			accepted <- conn
		}
		close(accepted)
	}()
	client, err = net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	server, ok := <-accepted
	require.True(t, ok, "listener accepts the client")
	t.Cleanup(func() { _ = server.Close() })
	return client, server
}
