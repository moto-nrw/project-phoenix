package postgres_test

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Keep real PostgreSQL but hold LISTEN packets at the transport boundary.
// A successful socket write is not proof that the server subscribed the API.
func TestRealtimeHubWaitsForListenerReadiness(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var database string
	require.NoError(t, db.DB.QueryRowContext(context.Background(), "SELECT current_database()").Scan(&database))
	listening := make(chan *heldListenConn, 1)
	dsn := os.Getenv("TEST_DB_DSN") // SetupTestDB resolves this required fixture value.
	require.NotEmpty(t, dsn)
	connector := pgdriver.NewConnector(pgdriver.WithDSN(dsn), pgdriver.WithDatabase(database), func(config *pgdriver.Config) {
		dial := config.Dialer
		config.Dialer = func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := dial(ctx, network, address)
			if err != nil {
				return nil, err
			}
			return &heldListenConn{Conn: conn, listening: listening}, nil
		}
	})
	delayed := bun.NewDB(sql.OpenDB(connector), db.Dialect())
	defer func() { require.NoError(t, delayed.Close()) }()
	type result struct {
		hub    *realtime.Hub
		closer io.Closer
		err    error
	}
	constructed := make(chan result, 1)
	go func() {
		hub, closer, err := deliveryCompose.NewPostgresRealtimeHub(delayed, slog.Default(), true)
		constructed <- result{hub, closer, err}
	}()
	var held *heldListenConn
	select {
	case held = <-listening:
	case <-time.After(3 * time.Second):
		t.Fatal("constructor never attempted LISTEN")
	}
	defer func() { _ = held.release() }()
	select {
	case early := <-constructed:
		require.NoError(t, held.release())
		if early.closer != nil {
			require.NoError(t, early.closer.Close())
		}
		t.Fatalf("API constructor returned before LISTEN reached PostgreSQL: %v", early.err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, held.release())
	var api result
	select {
	case api = <-constructed:
	case <-time.After(3 * time.Second):
		t.Fatal("API did not become ready after releasing LISTEN")
	}
	require.NoError(t, api.err)
	defer func() { require.NoError(t, api.closer.Close()) }()
	client := subscribeTransport(t, api.hub, testpkg.Tenant(t))
	worker := transportHub(t, db, false)
	require.NoError(t, worker.BroadcastToTenant(client.TenantID, realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{})))
	require.Equal(t, realtime.EventStaffTimeTrackingChanged, receiveTransportEvent(t, client).Type)
}

type heldListenConn struct {
	net.Conn
	mu        sync.Mutex
	pending   []byte
	holding   bool
	released  bool
	once      sync.Once
	listening chan<- *heldListenConn
}

func (conn *heldListenConn) Write(data []byte) (int, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if !conn.released && bytes.Contains(data, []byte(`LISTEN "moto_realtime_v1"`)) {
		conn.holding = true
	}
	if conn.holding {
		conn.pending = append(conn.pending, data...)
		conn.once.Do(func() { conn.listening <- conn })
		return len(data), nil
	}
	return conn.Conn.Write(data)
}

func (conn *heldListenConn) release() error {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if conn.released {
		return nil
	}
	conn.released, conn.holding = true, false
	_, err := io.Copy(conn.Conn, bytes.NewReader(conn.pending))
	conn.pending = nil
	return err
}
