package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	postgresadapter "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// listeningWakeups returns the wake-ups of db once its listener receives
// notifications, so a test cannot enqueue before LISTEN took effect.
func listeningWakeups(t *testing.T, db *bun.DB) <-chan struct{} {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	wakeups := postgresadapter.ListenEmailEnqueued(ctx, db, nil)
	deadline := time.After(10 * time.Second)
	for {
		_, err := db.ExecContext(ctx, "NOTIFY "+postgresadapter.EmailEnqueuedChannel)
		require.NoError(t, err)
		select {
		case <-wakeups:
			return wakeups
		case <-time.After(100 * time.Millisecond):
		case <-deadline:
			t.Fatal("listener never received a notification")
		}
	}
}

func requireWakeup(t *testing.T, wakeups <-chan struct{}) {
	t.Helper()
	select {
	case <-wakeups:
	case <-time.After(5 * time.Second):
		t.Fatal("committed enqueue did not wake the worker")
	}
}

func requireNoWakeup(t *testing.T, wakeups <-chan struct{}) {
	t.Helper()
	select {
	case <-wakeups:
		t.Fatal("worker was woken without a new committed entry")
	case <-time.After(500 * time.Millisecond):
	}
}

func TestEnqueuedEmailWakesListenerOnCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	wakeups := listeningWakeups(t, db)

	enqueueEmail(t, db, store, ctx, "wake:committed")

	requireWakeup(t, wakeups)
}

func TestEnqueuedEmailDoesNotWakeListenerOnRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	wakeups := listeningWakeups(t, db)
	rollback := errors.New("rollback")
	key := "wake:rolled-back"

	err := tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if _, err := store.Enqueue(txCtx, domain.Intent{
			TenantID: testpkg.Tenant(t), Transport: domain.TransportEmail, Template: "welcome", IdempotencyKey: &key,
			Recipient: []byte(`{"address":"guardian@example.com"}`), Payload: []byte(`{"name":"Ada"}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		}); err != nil {
			return err
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	requireNoWakeup(t, wakeups)
}

func TestDuplicateEnqueueDoesNotWakeListener(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	enqueueEmail(t, db, store, ctx, "wake:duplicate")
	wakeups := listeningWakeups(t, db)

	duplicate := enqueueEmail(t, db, store, ctx, "wake:duplicate")

	require.True(t, duplicate.Duplicate)
	requireNoWakeup(t, wakeups)
}

func TestEmailWakeupsCloseWithTheirContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	wakeups := postgresadapter.ListenEmailEnqueued(ctx, db, nil)

	cancel()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case _, open := <-wakeups:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("wake-ups stayed open after their context ended")
		}
	}
}
