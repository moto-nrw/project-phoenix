package postgres_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestWorkerSignalReachesEveryAPIOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	worker := transportHub(t, db, false)
	apiA, apiB := transportHub(t, db, true), transportHub(t, db, true)
	schoolID := testpkg.Tenant(t)
	a, b := subscribeTransport(t, apiA, schoolID), subscribeTransport(t, apiB, schoolID)
	foreign := subscribeTransport(t, apiA, testpkg.NewTenantScope(t, db).TenantID)
	require.NoError(t, tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		deliveryCompose.QueueStaffTimeTrackingChanged(ctx, worker, slog.Default())
		require.Empty(t, a.Channel, "uncommitted signal escaped")
		require.Empty(t, b.Channel, "uncommitted signal escaped")
		return nil
	}))
	for _, client := range []*realtime.Client{a, b} {
		select {
		case event := <-client.Channel:
			require.Equal(t, realtime.EventStaffTimeTrackingChanged, event.Type)
		case <-time.After(3 * time.Second):
			t.Fatal("committed Worker signal did not reach API subscriber")
		}
	}
	require.Empty(t, foreign.Channel, "foreign school received the committed signal")
}

func TestWorkerRollbackNeverPublishes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	worker, api := transportHub(t, db, false), transportHub(t, db, true)
	schoolID := testpkg.Tenant(t)
	client := subscribeTransport(t, api, schoolID)
	rollback := errors.New("job rolled back")
	err := tenant.WithTenantTx(testpkg.WithTenantRuntime(t, context.Background(), db), db, schoolID, func(ctx context.Context, _ bun.Tx) error {
		deliveryCompose.QueueStaffTimeTrackingChanged(ctx, worker, slog.Default())
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	// A subsequent committed signal is a delivery barrier, not a sleep-based absence check.
	marker := realtime.NewEvent(realtime.EventInstanceCompleted, "", realtime.EventData{})
	require.NoError(t, worker.BroadcastToTenant(schoolID, marker))
	select {
	case got := <-client.Channel:
		require.Equal(t, marker.Type, got.Type, "rolled-back job published a signal")
	case <-time.After(3 * time.Second):
		t.Fatal("delivery barrier did not arrive")
	}
	require.Empty(t, client.Channel)
}

func TestWorkerPublishFailureDoesNotDeliverLocally(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	worker := transportHub(t, db, false)
	client := subscribeTransport(t, worker, testpkg.Tenant(t))
	require.NoError(t, db.Close())
	err := worker.BroadcastToTenant(client.TenantID, realtime.NewEvent(realtime.EventInstanceStarted, "", realtime.EventData{}))
	require.Error(t, err)
	require.Empty(t, client.Channel, "failed transport must not fall back to a different audience")
}

func transportHub(t *testing.T, db *bun.DB, listen bool) *realtime.Hub {
	t.Helper()
	hub, closer, err := deliveryCompose.NewPostgresRealtimeHub(db, slog.Default(), listen)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closer.Close()) })
	return hub
}

func subscribeTransport(t *testing.T, hub *realtime.Hub, schoolID int64) *realtime.Client {
	t.Helper()
	client := &realtime.Client{Channel: make(chan realtime.Event, 16)}
	hub.Register(client, schoolID, nil)
	t.Cleanup(func() { hub.Unregister(client) })
	return client
}

func TestTransportKeepsGroupAndAccountAudiences(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	worker, api := transportHub(t, db, false), transportHub(t, db, true)
	schoolID := testpkg.Tenant(t)
	addressed := testpkg.CreateTestAccount(t, db, "realtime-addressed@example.com")
	bystander := testpkg.CreateTestAccount(t, db, "realtime-bystander@example.com")
	target := &realtime.Client{Channel: make(chan realtime.Event, 16), AccountID: addressed.ID, SubscribedGroups: make(map[string]bool)}
	other := &realtime.Client{Channel: make(chan realtime.Event, 16), AccountID: bystander.ID, SubscribedGroups: make(map[string]bool)}
	api.Register(target, schoolID, []string{"worker-group"})
	api.Register(other, schoolID, []string{"different-group"})
	t.Cleanup(func() { api.Unregister(target); api.Unregister(other) })
	for _, broadcast := range []func() error{
		func() error {
			return worker.BroadcastToGroup(schoolID, "worker-group", realtime.NewEvent(realtime.EventActivityEnd, "worker-group", realtime.EventData{}))
		},
		func() error {
			return worker.BroadcastToStaffAccounts(schoolID, []int64{addressed.ID}, realtime.NewEvent(realtime.EventNotification, "", realtime.EventData{}))
		},
	} {
		require.NoError(t, broadcast())
		select {
		case <-target.Channel:
		case <-time.After(3 * time.Second):
			t.Fatal("addressed subscriber did not receive signal")
		}
		require.Empty(t, other.Channel, "audience broadened to an unaddressed subscriber")
	}
}
