package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// fakePushRepo implements the subscription lookups the channel needs; the
// embedded interface panics on anything else (nothing else may be called).
type fakePushRepo struct {
	iot.PushSubscriptionRepository
	staff     []*iot.PushSubscription
	admins    []*iot.PushSubscription
	guardians map[int64][]*iot.PushSubscription
	deleted   []any
	deleteErr error
	mu        sync.Mutex
}

func (f *fakePushRepo) FindForTenantStaff(context.Context) ([]*iot.PushSubscription, error) {
	return f.staff, nil
}

func (f *fakePushRepo) FindForTenantAdmins(context.Context) ([]*iot.PushSubscription, error) {
	return f.admins, nil
}

func (f *fakePushRepo) FindForGuardian(_ context.Context, accountID int64) ([]*iot.PushSubscription, error) {
	return f.guardians[accountID], nil
}

func (f *fakePushRepo) Delete(_ context.Context, id any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

// fakeSender records every push request and replies with a scripted status
// per endpoint (default 201).
type fakeSender struct {
	statusByEndpoint map[string]int
	errorByEndpoint  map[string]error
	sent             []sentPush
	sendFn           func(context.Context)
	done             chan struct{}
	mu               sync.Mutex
}

type sentPush struct {
	endpoint string
	payload  []byte
	options  *webpush.Options
}

func (f *fakeSender) Send(ctx context.Context, sub *webpush.Subscription, payload []byte, opts *webpush.Options) (*http.Response, error) {
	if f.sendFn != nil {
		f.sendFn(ctx)
	}
	f.mu.Lock()
	f.sent = append(f.sent, sentPush{endpoint: sub.Endpoint, payload: payload, options: opts})
	f.mu.Unlock()
	if f.done != nil {
		close(f.done)
	}
	if err := f.errorByEndpoint[sub.Endpoint]; err != nil {
		return nil, err
	}
	status := f.statusByEndpoint[sub.Endpoint]
	if status == 0 {
		status = http.StatusCreated
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func testVAPID() VAPIDConfig {
	return VAPIDConfig{PublicKey: "pub", PrivateKey: "priv", Subscriber: "mailto:test@example.org"}
}

func testSub(id, tenantID int64, endpoint string) *iot.PushSubscription {
	sub := &iot.PushSubscription{
		AccountID: 7,
		Portal:    iot.PushPortalStaff,
		Endpoint:  endpoint,
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
	}
	sub.ID = id
	sub.TenantID = tenantID
	return sub
}

func testChannel(repo *fakePushRepo, sender *fakeSender) *webPushChannel {
	return &webPushChannel{
		repo:   repo,
		vapid:  testVAPID(),
		sender: sender,
		logger: slog.Default(),
	}
}

func TestWebPushPayloadIsGDPRSafe(t *testing.T) {
	repo := &fakePushRepo{staff: []*iot.PushSubscription{testSub(1, 41, "https://fcm.googleapis.com/a")}}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)

	event := Event{
		Type:     "reminders_due",
		Audience: Audience{TenantID: 41, Scope: ScopeTenant},
		Priority: PriorityHigh,
		Title:    "3 Erinnerungen",
		Body:     "Es stehen Abholungen an.",
		DeepLink: "/reminders",
		Data:     map[string]string{"student_name": "MUST-NEVER-LEAVE-THE-APP"},
	}

	payload, err := json.Marshal(webPushPayload{
		Title:    event.Title,
		Body:     event.Body,
		DeepLink: event.DeepLink,
		Type:     event.Type,
		Priority: event.Priority,
	})
	require.NoError(t, err)
	channel.sendAll(context.Background(), event, payload, repo.staff)

	require.Len(t, sender.sent, 1)
	var wire map[string]any
	require.NoError(t, json.Unmarshal(sender.sent[0].payload, &wire))
	assert.Equal(t, "3 Erinnerungen", wire["title"])
	assert.Equal(t, "/reminders", wire["deepLink"])
	// The GDPR contract: Data must never reach the push service.
	assert.NotContains(t, string(sender.sent[0].payload), "MUST-NEVER-LEAVE-THE-APP")
	assert.NotContains(t, wire, "data")
	// Urgency/TTL mapping for high priority.
	assert.Equal(t, webpush.UrgencyHigh, sender.sent[0].options.Urgency)
	assert.Equal(t, 3600, sender.sent[0].options.TTL)
	assert.Equal(t, "mailto:test@example.org", sender.sent[0].options.Subscriber)
}

func TestWebPushPrunesExpiredSubscriptions(t *testing.T) {
	dead := testSub(11, 41, "https://fcm.googleapis.com/dead")
	alive := testSub(12, 41, "https://fcm.googleapis.com/alive")
	repo := &fakePushRepo{staff: []*iot.PushSubscription{dead, alive}}
	sender := &fakeSender{statusByEndpoint: map[string]int{"https://fcm.googleapis.com/dead": http.StatusGone}}
	channel := testChannel(repo, sender)

	event := Event{Type: "test", Audience: Audience{TenantID: 41, Scope: ScopeTenant}, Priority: PriorityNormal, Title: "t"}
	channel.sendAll(context.Background(), event, []byte(`{"title":"t"}`), repo.staff)

	// Both were attempted; only the dead one was pruned.
	require.Len(t, sender.sent, 2)
	require.Len(t, repo.deleted, 1)
	assert.Equal(t, int64(11), repo.deleted[0])
}

func TestWebPushResolveSubscriptionsScopes(t *testing.T) {
	staff := []*iot.PushSubscription{testSub(1, 41, "https://fcm.googleapis.com/s")}
	admins := []*iot.PushSubscription{testSub(2, 41, "https://fcm.googleapis.com/a")}
	guardian := testSub(3, 41, "https://fcm.googleapis.com/g")
	guardian.Portal = iot.PushPortalParent
	repo := &fakePushRepo{
		staff:     staff,
		admins:    admins,
		guardians: map[int64][]*iot.PushSubscription{99: {guardian}},
	}
	channel := testChannel(repo, &fakeSender{})

	got, err := channel.resolveSubscriptions(context.Background(), Audience{TenantID: 41, Scope: ScopeTenant})
	require.NoError(t, err)
	assert.Equal(t, staff, got)

	got, err = channel.resolveSubscriptions(context.Background(), Audience{TenantID: 41, Scope: ScopeAdmin})
	require.NoError(t, err)
	assert.Equal(t, admins, got)

	got, err = channel.resolveSubscriptions(context.Background(), Audience{TenantID: 41, Scope: ScopeGuardian, GuardianAccountID: 99})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{guardian}, got)

	// Group scope is deliberately unsupported: no error, no recipients.
	got, err = channel.resolveSubscriptions(context.Background(), Audience{TenantID: 41, Scope: ScopeGroup, ActiveGroupID: "g1"})
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = channel.resolveSubscriptions(context.Background(), Audience{TenantID: 41, Scope: "bogus"})
	require.Error(t, err)
}

func TestWebPushPriorityMapping(t *testing.T) {
	ttl, urgency := pushOptionsForPriority(PriorityHigh)
	assert.Equal(t, 3600, ttl)
	assert.Equal(t, webpush.UrgencyHigh, urgency)

	ttl, urgency = pushOptionsForPriority(PriorityNormal)
	assert.Equal(t, 86400, ttl)
	assert.Equal(t, webpush.UrgencyNormal, urgency)

	ttl, urgency = pushOptionsForPriority(PriorityLow)
	assert.Equal(t, 86400, ttl)
	assert.Equal(t, webpush.UrgencyLow, urgency)
}

func TestWebPushHandlesDeliveryFailures(t *testing.T) {
	sendFailure := testSub(21, 41, "https://fcm.googleapis.com/send-failure")
	pruneFailure := testSub(22, 41, "https://fcm.googleapis.com/prune-failure")
	rejected := testSub(23, 41, "https://fcm.googleapis.com/rejected")
	repo := &fakePushRepo{
		deleteErr: errors.New("delete failed"),
	}
	sender := &fakeSender{
		statusByEndpoint: map[string]int{
			pruneFailure.Endpoint: http.StatusGone,
			rejected.Endpoint:     http.StatusBadRequest,
		},
		errorByEndpoint: map[string]error{
			sendFailure.Endpoint: errors.New("send failed"),
		},
	}
	channel := testChannel(repo, sender)
	event := Event{Type: "test", Audience: Audience{TenantID: 41}, Priority: PriorityNormal}

	channel.sendAll(context.Background(), event, []byte(`{"title":"test"}`), []*iot.PushSubscription{
		sendFailure,
		pruneFailure,
		rejected,
	})
	channel.handleResponse(context.Background(), event, rejected, nil)

	require.Len(t, sender.sent, 3)
	assert.Equal(t, []any{int64(22)}, repo.deleted)
}

func TestWebPushRejectsOversizedPayload(t *testing.T) {
	channel := testChannel(&fakePushRepo{}, &fakeSender{})
	event := Event{
		Type:     "test",
		Audience: Audience{TenantID: 41, Scope: ScopeTenant},
		Title:    strings.Repeat("x", maxPushPayloadBytes),
	}

	err := channel.Deliver(context.Background(), event)

	require.Error(t, err)
	assert.ErrorContains(t, err, "web push payload exceeds")
}

func TestWebPushRejectsUntrustedStoredEndpoint(t *testing.T) {
	repo := &fakePushRepo{}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)
	event := Event{Type: "test", Audience: Audience{TenantID: 41}, Priority: PriorityNormal}

	channel.sendAll(context.Background(), event, []byte(`{"title":"test"}`), []*iot.PushSubscription{
		testSub(1, 41, "https://127.0.0.1/internal"),
	})

	assert.Empty(t, sender.sent)
}

func TestWebPushSendDeadlineAndRedirectPolicy(t *testing.T) {
	var deadline time.Time
	sender := &fakeSender{sendFn: func(ctx context.Context) {
		deadline, _ = ctx.Deadline()
	}}
	channel := testChannel(&fakePushRepo{}, sender)
	event := Event{Type: "test", Audience: Audience{TenantID: 41}, Priority: PriorityNormal}

	channel.sendAll(context.Background(), event, []byte(`{"title":"test"}`), []*iot.PushSubscription{
		testSub(1, 41, "https://fcm.googleapis.com/device"),
	})

	require.False(t, deadline.IsZero())
	assert.WithinDuration(t, time.Now().Add(pushSendTimeout), deadline, time.Second)

	client := newWebPushHTTPClient()
	req, err := http.NewRequest(http.MethodGet, "https://fcm.googleapis.com/next", nil)
	require.NoError(t, err)
	assert.ErrorIs(t, client.CheckRedirect(req, nil), http.ErrUseLastResponse)
	assert.Equal(t, pushSendTimeout, client.Timeout)
}

func TestWebPushDeliverCommitsBeforeAsyncSend(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = db.Close()
		_ = sqlDB.Close()
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	sendStarted := make(chan error, 1)
	sendFinished := make(chan struct{})
	releaseSend := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseSend) }) })
	sender := &fakeSender{
		sendFn: func(context.Context) {
			sendStarted <- mock.ExpectationsWereMet()
			<-releaseSend
		},
		done: sendFinished,
	}
	repo := &fakePushRepo{staff: []*iot.PushSubscription{
		testSub(1, 41, "https://fcm.googleapis.com/device"),
	}}
	channel := testChannel(repo, sender)
	channel.db = db

	deliveryDone := make(chan error, 1)
	go func() {
		deliveryDone <- channel.Deliver(context.Background(), Event{
			Type:     "test",
			Audience: Audience{TenantID: 41, Scope: ScopeTenant},
			Priority: PriorityNormal,
			Title:    "Test",
		})
	}()

	select {
	case sendErr := <-sendStarted:
		require.NoError(t, sendErr, "the tenant transaction must commit before the HTTP send starts")
	case <-time.After(time.Second):
		t.Fatal("web push send did not start")
	}

	select {
	case deliverErr := <-deliveryDone:
		require.NoError(t, deliverErr, "Deliver must not wait for a slow push service")
	case <-time.After(time.Second):
		t.Fatal("Deliver waited for the outbound push request")
	}

	releaseOnce.Do(func() { close(releaseSend) })
	select {
	case <-sendFinished:
	case <-time.After(time.Second):
		t.Fatal("web push send did not finish")
	}
}
