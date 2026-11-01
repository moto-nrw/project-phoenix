package postgres_test

import (
	"context"
	"encoding/json"
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
		// Publishing a marker through the SAME FIFO publisher is a receive barrier:
		// an incorrectly immediate hook would arrive before it, before commit.
		require.NoError(t, worker.BroadcastToTenant(schoolID, realtime.NewEvent(realtime.EventInstanceStarted, "", realtime.EventData{})))
		for _, client := range []*realtime.Client{a, b} {
			select {
			case event := <-client.Channel:
				require.Equal(t, realtime.EventInstanceStarted, event.Type, "uncommitted signal escaped before barrier")
			case <-time.After(3 * time.Second):
				t.Fatal("pre-commit delivery barrier did not arrive")
			}
		}
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

func TestWorkerPublishFailureStillDeliversLocally(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	worker := transportHub(t, db, false)
	client := subscribeTransport(t, worker, testpkg.Tenant(t))
	require.NoError(t, db.Close())
	err := worker.BroadcastToTenant(client.TenantID, realtime.NewEvent(realtime.EventInstanceStarted, "", realtime.EventData{}))
	require.NoError(t, err)
	require.Equal(t, realtime.EventInstanceStarted, receiveTransportEvent(t, client).Type)
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
			return worker.BroadcastToStaffAccounts(schoolID, []int64{addressed.ID}, realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{}))
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

func TestRelayCarriesOnlyInvalidationFields(t *testing.T) {
	t.Parallel()
	schoolID := testpkg.Tenant(t)
	var payload string
	hub, err := realtime.NewRelayedHub(slog.Default(), func(s string) error { payload = s; return nil })
	require.NoError(t, err)
	content := "private content and /untrusted-link"
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{
		ActivityName: &content, AttendanceNote: &content, Title: &content, Body: &content, DeepLink: &content, Source: &content, Reason: &content,
	})
	require.NoError(t, hub.BroadcastToTenant(schoolID, event))
	require.NotEmpty(t, payload)
	require.NotContains(t, payload, content)
	var wire map[string]any
	require.NoError(t, json.Unmarshal([]byte(payload), &wire))
	require.Equal(t, "student_updated", wire["type"])
	require.NotContains(t, wire, "event")
	require.Len(t, wire, 5, "wire must contain only school, audience, origin, type and invalidation IDs")
	require.Empty(t, wire["ids"])
}

func TestForgedSignalCannotDisplayContent(t *testing.T) {
	t.Parallel()
	schoolID := testpkg.Tenant(t)
	hub, err := realtime.NewRelayedHub(slog.Default(), func(string) error { return nil })
	require.NoError(t, err)
	client := subscribeTransport(t, hub, schoolID)
	payload, err := json.Marshal(map[string]any{
		"school_id": schoolID, "audience": "tenant", "origin": "foreign-process", "type": "notification",
		"event": realtime.NewEvent(realtime.EventNotification, "", realtime.EventData{Title: stringPointer("forged title"), DeepLink: stringPointer("/forged")}),
	})
	require.NoError(t, err)
	require.Error(t, hub.ReceiveSignal(string(payload)))
	require.Empty(t, client.Channel)
}

func stringPointer(s string) *string { return &s }

func TestOwnSignalIsNotDeliveredTwice(t *testing.T) {
	t.Parallel()
	var payload string
	hub, err := realtime.NewRelayedHub(slog.Default(), func(s string) error { payload = s; return nil })
	require.NoError(t, err)
	client := subscribeTransport(t, hub, testpkg.Tenant(t))
	require.NoError(t, hub.BroadcastToTenant(client.TenantID, realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{})))
	require.Len(t, client.Channel, 1, "local delivery must precede the listener")
	require.NoError(t, hub.ReceiveSignal(payload))
	require.Len(t, client.Channel, 1, "own listener echo duplicated the event")
}

func TestNotificationsStayLocal(t *testing.T) {
	t.Parallel()
	var published bool
	hub, err := realtime.NewRelayedHub(slog.Default(), func(string) error { published = true; return nil })
	require.NoError(t, err)
	client := subscribeTransport(t, hub, testpkg.Tenant(t))
	event := realtime.NewEvent(realtime.EventNotification, "", realtime.EventData{Title: stringPointer("local title")})
	require.NoError(t, hub.BroadcastToTenant(client.TenantID, event))
	require.False(t, published)
	require.Equal(t, event, receiveTransportEvent(t, client))
}

func TestForgedInvalidationDropsDisplayFields(t *testing.T) {
	t.Parallel()
	hub, err := realtime.NewRelayedHub(slog.Default(), func(string) error { return nil })
	require.NoError(t, err)
	client := subscribeTransport(t, hub, testpkg.Tenant(t))
	payload, err := json.Marshal(map[string]any{
		"school_id": client.TenantID, "audience": "tenant", "origin": "foreign-process", "type": "student_updated",
		"ids":   map[string]any{"title": "forged title", "deep_link": "/forged", "reason": "forged text", "student_id": "forged text"},
		"event": realtime.NewEvent(realtime.EventNotification, "", realtime.EventData{Title: stringPointer("forged title")}),
	})
	require.NoError(t, err)
	require.NoError(t, hub.ReceiveSignal(string(payload)))
	event := receiveTransportEvent(t, client)
	require.Equal(t, realtime.EventStudentUpdated, event.Type)
	require.Equal(t, realtime.EventData{}, event.Data)
}

func TestPublishQueueDoesNotWaitForPoolAndHasBound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	hub := transportHub(t, db, false)
	db.SetMaxOpenConns(1)
	conn, err := db.DB.Conn(context.Background())
	require.NoError(t, err)
	defer func() { require.NoError(t, conn.Close()) }()
	schoolID := testpkg.Tenant(t)
	result := make(chan error, 1)
	go func() {
		result <- hub.BroadcastToTenant(schoolID, realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{}))
	}()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("after-commit hook blocked on the database pool")
	}
	overflow := false
	for range 130 {
		err := hub.BroadcastToTenant(schoolID, realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{}))
		if err != nil {
			overflow = true
		}
	}
	require.True(t, overflow, "publisher must bound queued signals")
}

func TestLocalDeliverySurvivesPublisherError(t *testing.T) {
	t.Parallel()
	hub, err := realtime.NewRelayedHub(slog.Default(), func(string) error { return errors.New("NOTIFY unavailable") })
	require.NoError(t, err)
	client := subscribeTransport(t, hub, testpkg.Tenant(t))
	other := subscribeTransport(t, hub, testpkg.NewTenantScope(t, testpkg.SetupTestDB(t)).TenantID)
	event := realtime.NewEvent(realtime.EventStaffTimeTrackingChanged, "", realtime.EventData{})
	require.ErrorContains(t, hub.BroadcastToTenant(client.TenantID, event), "NOTIFY unavailable")
	require.Equal(t, event, receiveTransportEvent(t, client))
	require.Empty(t, other.Channel)
}

func receiveTransportEvent(t *testing.T, client *realtime.Client) realtime.Event {
	t.Helper()
	select {
	case event := <-client.Channel:
		return event
	case <-time.After(3 * time.Second):
		t.Fatal("subscriber received no event")
		return realtime.Event{}
	}
}
