package notifications

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// scriptedSender answers each endpoint with a canned outcome. Unlike fakeSender
// it can also return the (nil, nil) pair a misbehaving transport produces,
// which the synchronous path has to survive.
type scriptedSender struct {
	respond func(endpoint string) (*http.Response, error)
}

func (s scriptedSender) Send(_ context.Context, sub *webpush.Subscription, _ []byte, _ *webpush.Options) (*http.Response, error) {
	return s.respond(sub.Endpoint)
}

func okResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}
}

func guardianSyncEvent() Event {
	return Event{
		Type:     TypeParentAppointmentReminder,
		Audience: Audience{TenantID: 41, Scope: ScopeGuardian, GuardianAccountIDs: []int64{77}},
		Priority: PriorityNormal,
		Title:    "Terminerinnerung",
	}
}

// The synchronous path exists so a durable producer learns whether the push was
// accepted. "Nobody to deliver to" must be distinguishable from "delivery
// failed": the first is final, the second is worth retrying.
func TestWebPushDeliverSynchronouslyReportsMissingSubscribers(t *testing.T) {
	t.Parallel()

	t.Run("without VAPID keys there is no push service to accept anything", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.vapid = VAPIDConfig{}

		require.ErrorIs(t, channel.DeliverSynchronously(context.Background(), guardianSyncEvent()),
			ErrNoWebPushSubscribers)
	})

	t.Run("an audience without registered devices is not a delivery failure", func(t *testing.T) {
		db, mock := mockTenantTx(t)
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.db = db
		channel.SetTenantRuntime(newMockTenantRuntime(t, db))

		require.ErrorIs(t, channel.DeliverSynchronously(context.Background(), guardianSyncEvent()),
			ErrNoWebPushSubscribers)
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("an oversized payload is refused before any transaction opens", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		event := guardianSyncEvent()
		event.Body = strings.Repeat("x", maxPushPayloadBytes+1)

		err := channel.DeliverSynchronously(context.Background(), event)
		require.Error(t, err)
		assert.NotErrorIs(t, err, ErrNoWebPushSubscribers,
			"a payload bug must not read as an empty audience")
	})

	t.Run("a failed subscription lookup is reported, not silently empty", func(t *testing.T) {
		sqlDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		db := bun.NewDB(sqlDB, pgdialect.New())
		t.Cleanup(func() {
			_ = sqlDB.Close()
		})
		beginErr := errors.New("connection lost")
		mock.ExpectBegin().WillReturnError(beginErr)

		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.db = db
		channel.SetTenantRuntime(newMockTenantRuntime(t, db))

		require.ErrorIs(t, channel.DeliverSynchronously(context.Background(), guardianSyncEvent()), beginErr)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestWebPushDeliverSynchronouslySendsToRegisteredDevices(t *testing.T) {
	t.Parallel()

	db, mock := mockTenantTx(t)
	guardian := testSub(3, 41, "https://fcm.googleapis.com/g")
	guardian.Portal = iot.PushPortalParent
	repo := &fakePushRepo{guardians: map[int64][]*iot.PushSubscription{77: {guardian}}}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)
	channel.db = db
	channel.SetTenantRuntime(newMockTenantRuntime(t, db))

	require.NoError(t, channel.DeliverSynchronously(context.Background(), guardianSyncEvent()))

	require.Len(t, sender.sent, 1)
	assert.Equal(t, "https://fcm.googleapis.com/g", sender.sent[0].endpoint)
	require.NoError(t, mock.ExpectationsWereMet())
}

// One accepted device is enough: the reminder claim is per guardian, so a
// second device's failure must not make the producer retry and duplicate the
// notification on the device that already got it.
func TestWebPushSynchronousDispatchSucceedsIfAnyDeviceAccepts(t *testing.T) {
	t.Parallel()

	good := testSub(1, 41, "https://fcm.googleapis.com/good")
	bad := testSub(2, 41, "https://fcm.googleapis.com/bad")
	sender := &fakeSender{errorByEndpoint: map[string]error{
		"https://fcm.googleapis.com/bad": errors.New("push service unreachable"),
	}}
	channel := testChannel(&fakePushRepo{}, sender)

	err := channel.sendAllSynchronously(context.Background(), guardianSyncEvent(), []byte(`{}`),
		[]*iot.PushSubscription{good, bad})

	require.NoError(t, err)
}

func TestWebPushSynchronousDispatchReportsWhenEveryDeviceFails(t *testing.T) {
	t.Parallel()

	first := testSub(1, 41, "https://fcm.googleapis.com/first")
	second := testSub(2, 41, "https://fcm.googleapis.com/second")
	sendErr := errors.New("push service unreachable")
	sender := &fakeSender{errorByEndpoint: map[string]error{
		"https://fcm.googleapis.com/first":  sendErr,
		"https://fcm.googleapis.com/second": sendErr,
	}}
	channel := testChannel(&fakePushRepo{}, sender)

	err := channel.sendAllSynchronously(context.Background(), guardianSyncEvent(), []byte(`{}`),
		[]*iot.PushSubscription{first, second})

	require.ErrorIs(t, err, sendErr)
}

func TestWebPushSynchronousDispatchHandlesCancellation(t *testing.T) {
	t.Parallel()

	// A full slot channel forces the dispatch loop into its cancellation branch:
	// with no slot available, the only ready case is ctx.Done().
	fillSlots := func(c *webPushChannel) {
		for range cap(c.sendSlots) {
			c.sendSlots <- struct{}{}
		}
	}

	t.Run("a cancelled dispatch that sent nothing reports the cancellation", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		fillSlots(channel)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := channel.sendAllSynchronously(ctx, guardianSyncEvent(), []byte(`{}`),
			[]*iot.PushSubscription{testSub(1, 41, "https://fcm.googleapis.com/a")})

		require.ErrorIs(t, err, context.Canceled)
	})

	// Cancellation while a device is still in flight must not discard what that
	// device already told us: the in-flight result decides the outcome, not the
	// cancellation.
	cancelMidDispatch := func(t *testing.T, inFlightErr error) error {
		t.Helper()
		started := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		sender := &fakeSender{sendFn: func(context.Context) {
			once.Do(func() { close(started) })
			<-release
		}}
		if inFlightErr != nil {
			// Both devices share the outcome, so the assertion holds whether or
			// not the second one still slipped past the cancellation check.
			sender.errorByEndpoint = map[string]error{
				"https://fcm.googleapis.com/a": inFlightErr,
				"https://fcm.googleapis.com/b": inFlightErr,
			}
		}
		channel := testChannel(&fakePushRepo{}, sender)
		// Leave exactly one slot: the first device takes it, the second blocks.
		for range cap(channel.sendSlots) - 1 {
			channel.sendSlots <- struct{}{}
		}
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			done <- channel.sendAllSynchronously(ctx, guardianSyncEvent(), []byte(`{}`), []*iot.PushSubscription{
				testSub(1, 41, "https://fcm.googleapis.com/a"),
				testSub(2, 41, "https://fcm.googleapis.com/b"),
			})
		}()

		<-started
		cancel()
		close(release)
		return <-done
	}

	t.Run("a device that accepted before the cancellation still counts as delivered", func(t *testing.T) {
		require.NoError(t, cancelMidDispatch(t, nil))
	})

	t.Run("a cancelled dispatch whose in-flight device failed reports that failure", func(t *testing.T) {
		sendErr := errors.New("push service unreachable")
		require.ErrorIs(t, cancelMidDispatch(t, sendErr), sendErr)
	})

	t.Run("an empty device list is not an error on a live context", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		require.NoError(t, channel.sendAllSynchronously(context.Background(), guardianSyncEvent(), []byte(`{}`), nil))
	})
}

// sendOneSynchronously is where "accepted" is actually decided. Each branch
// below is a different answer to the producer's question "may I mark this
// reminder delivered?".
func TestWebPushSendOneSynchronouslyClassifiesResponses(t *testing.T) {
	t.Parallel()

	event := guardianSyncEvent()

	t.Run("an untrusted endpoint never reaches the network", func(t *testing.T) {
		sender := &fakeSender{}
		channel := testChannel(&fakePushRepo{}, sender)
		sub := testSub(1, 41, "https://evil.example.com/push")

		require.Error(t, channel.sendOneSynchronously(context.Background(), event, []byte(`{}`), sub, 60, webpush.UrgencyNormal))
		assert.Empty(t, sender.sent)
	})

	t.Run("a 2xx response is acceptance", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		sub := testSub(1, 41, "https://fcm.googleapis.com/ok")

		require.NoError(t, channel.sendOneSynchronously(context.Background(), event, []byte(`{}`), sub, 60, webpush.UrgencyNormal))
	})

	t.Run("a transport that answers with neither response nor error is a failure", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.sender = scriptedSender{respond: func(string) (*http.Response, error) { return nil, nil }}
		sub := testSub(1, 41, "https://fcm.googleapis.com/silent")

		err := channel.sendOneSynchronously(context.Background(), event, []byte(`{}`), sub, 60, webpush.UrgencyNormal)
		assert.EqualError(t, err, "web push service returned no response")
	})

	t.Run("an expired subscription is pruned and still reported as undelivered", func(t *testing.T) {
		repo := &fakePushRepo{}
		channel := testChannel(repo, &fakeSender{})
		channel.sender = scriptedSender{respond: func(string) (*http.Response, error) {
			return okResponse(http.StatusGone), nil
		}}
		sub := testSub(11, 41, "https://fcm.googleapis.com/expired")

		err := channel.sendOneSynchronously(context.Background(), event, []byte(`{}`), sub, 60, webpush.UrgencyNormal)
		assert.ErrorContains(t, err, "410")
		assert.Equal(t, []any{int64(11)}, repo.deleted,
			"a device the push service retired must not be tried again")
	})

	t.Run("a failed prune surfaces instead of being masked by the status", func(t *testing.T) {
		deleteErr := errors.New("delete failed")
		repo := &fakePushRepo{deleteErr: deleteErr}
		channel := testChannel(repo, &fakeSender{})
		channel.sender = scriptedSender{respond: func(string) (*http.Response, error) {
			return okResponse(http.StatusNotFound), nil
		}}
		sub := testSub(12, 41, "https://fcm.googleapis.com/gone")

		require.ErrorIs(t, channel.sendOneSynchronously(context.Background(), event, []byte(`{}`), sub, 60, webpush.UrgencyNormal),
			deleteErr)
	})

	t.Run("any other rejection is reported with its status", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.sender = scriptedSender{respond: func(string) (*http.Response, error) {
			return okResponse(http.StatusInternalServerError), nil
		}}
		sub := testSub(13, 41, "https://fcm.googleapis.com/broken")

		assert.ErrorContains(t,
			channel.sendOneSynchronously(context.Background(), event, []byte(`{}`), sub, 60, webpush.UrgencyNormal),
			"500")
	})
}
