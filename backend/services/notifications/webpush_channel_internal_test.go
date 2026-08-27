package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	staff              []*iot.PushSubscription
	admins             []*iot.PushSubscription
	guardians          map[int64][]*iot.PushSubscription
	deleted            []any
	deleteAttempts     []*iot.PushSubscription
	keepExpired        bool
	staffAccounts      map[int64][]*iot.PushSubscription
	schoolAccounts     map[int64][]*iot.PushSubscription
	staffAccountsAsked []int64
	staffAccountsErr   error
	deleteErr          error
	// guardiansStudentIDs records the children the guardian lookup was scoped
	// to, and hiddenStudentID is the child whose access the school has revoked.
	guardiansStudentIDs []int64
	hiddenStudentID     int64
	mu                  sync.Mutex
}

func (f *fakePushRepo) FindForTenantStaff(context.Context) ([]*iot.PushSubscription, error) {
	return f.staff, nil
}

func (f *fakePushRepo) FindForTenantAdmins(context.Context) ([]*iot.PushSubscription, error) {
	return f.admins, nil
}

func (f *fakePushRepo) FindForGuardians(_ context.Context, accountIDs []int64, studentIDs []int64) ([]*iot.PushSubscription, error) {
	f.guardiansStudentIDs = studentIDs
	// Mirrors the repository predicate: the scope admits an account only while it
	// still holds access to at least one of the listed children.
	if len(studentIDs) > 0 {
		permitted := false
		for _, studentID := range studentIDs {
			if studentID != f.hiddenStudentID {
				permitted = true
				break
			}
		}
		if !permitted {
			return nil, nil
		}
	}
	var subscriptions []*iot.PushSubscription
	for _, accountID := range accountIDs {
		subscriptions = append(subscriptions, f.guardians[accountID]...)
	}
	return subscriptions, nil
}

func (f *fakePushRepo) DeleteExpiredIfUnchanged(_ context.Context, sub *iot.PushSubscription) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteAttempts = append(f.deleteAttempts, sub)
	if f.deleteErr != nil {
		return false, f.deleteErr
	}
	if f.keepExpired {
		return false, nil
	}
	f.deleted = append(f.deleted, sub.ID)
	return true, nil
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
		repo:      repo,
		vapid:     testVAPID(),
		sender:    sender,
		logger:    slog.Default(),
		sendSlots: make(chan struct{}, maxConcurrentPushSends),
	}
}

func TestWebPushPayloadIsGDPRSafe(t *testing.T) {
	t.Parallel()

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
	assert.Equal(t, "test@example.org", sender.sent[0].options.Subscriber)
}

func TestWebPushPrunesExpiredSubscriptions(t *testing.T) {
	t.Parallel()

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

func TestWebPushKeepsSubscriptionRefreshedDuringDelivery(t *testing.T) {
	t.Parallel()

	sentSnapshot := testSub(11, 41, "https://fcm.googleapis.com/refreshed")
	repo := &fakePushRepo{keepExpired: true}
	channel := testChannel(repo, &fakeSender{})
	event := Event{Type: "test", Audience: Audience{TenantID: 41, Scope: ScopeTenant}, Priority: PriorityNormal, Title: "t"}

	channel.handleResponse(context.Background(), event, sentSnapshot, &http.Response{
		StatusCode: http.StatusGone,
		Body:       io.NopCloser(strings.NewReader("")),
	})

	require.Len(t, repo.deleteAttempts, 1)
	assert.Same(t, sentSnapshot, repo.deleteAttempts[0])
	assert.Empty(t, repo.deleted)
}

func TestWebPushResolveSubscriptionsScopes(t *testing.T) {
	t.Parallel()

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

	got, err := channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{TenantID: 41, Scope: ScopeTenant}})
	require.NoError(t, err)
	assert.Equal(t, staff, got)

	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{TenantID: 41, Scope: ScopeAdmin}})
	require.NoError(t, err)
	assert.Equal(t, admins, got)

	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{TenantID: 41, Scope: ScopeGuardian, GuardianAccountID: 99}})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{guardian}, got)

	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{
		TenantID:           41,
		Scope:              ScopeGuardian,
		GuardianAccountIDs: []int64{99},
	}})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{guardian}, got)

	// A child-scoped audience carries the child into the device lookup, which is
	// where parent_portal.access is answered for the transaction that sends.
	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{
		TenantID:           41,
		Scope:              ScopeGuardian,
		GuardianAccountIDs: []int64{99},
		StudentIDs:         []int64{55},
	}})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{guardian}, got)
	assert.Equal(t, []int64{55}, repo.guardiansStudentIDs,
		"the lookup must be told which children the notification is about")

	// Access to that child revoked since the producer picked its audience: the
	// account keeps its devices and its consent, and still gets nothing.
	repo.hiddenStudentID = 55
	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{
		TenantID:           41,
		Scope:              ScopeGuardian,
		GuardianAccountIDs: []int64{99},
		StudentIDs:         []int64{55},
	}})
	require.NoError(t, err)
	assert.Empty(t, got)

	// An event that is not about one child stays unscoped.
	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{
		TenantID:           41,
		Scope:              ScopeGuardian,
		GuardianAccountIDs: []int64{99},
	}})
	require.NoError(t, err)
	assert.Equal(t, []*iot.PushSubscription{guardian}, got)
	assert.Empty(t, repo.guardiansStudentIDs)
	repo.hiddenStudentID = 0

	// Group scope is deliberately unsupported: no error, no recipients.
	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{TenantID: 41, Scope: ScopeGroup, ActiveGroupID: "g1"}})
	require.NoError(t, err)
	assert.Empty(t, got)

	_, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{TenantID: 41, Scope: "bogus"}})
	require.Error(t, err)
}

func TestWebPushPriorityMapping(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	require.Len(t, repo.deleteAttempts, 1)
	assert.Equal(t, int64(22), repo.deleteAttempts[0].ID)
	assert.Empty(t, repo.deleted)
}

func TestWebPushRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

func TestWebPushConcurrencyLimitIsSharedAcrossBatches(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	started := make(chan struct{}, maxConcurrentPushSends*2)
	sender := &fakeSender{sendFn: func(context.Context) {
		started <- struct{}{}
		<-release
	}}
	channel := testChannel(&fakePushRepo{}, sender)
	event := Event{Type: "test", Audience: Audience{TenantID: 41}, Priority: PriorityNormal}
	subscriptions := make([]*iot.PushSubscription, maxConcurrentPushSends)
	for i := range subscriptions {
		subscriptions[i] = testSub(int64(i+1), 41, fmt.Sprintf("https://fcm.googleapis.com/device-%d", i))
	}

	var batches sync.WaitGroup
	batches.Add(2)
	for range 2 {
		go func() {
			defer batches.Done()
			channel.sendAll(context.Background(), event, []byte(`{"title":"test"}`), subscriptions)
		}()
	}

	for range maxConcurrentPushSends {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("shared concurrency slots were not filled")
		}
	}
	select {
	case <-started:
		t.Fatalf("more than %d sends started concurrently across batches", maxConcurrentPushSends)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	batches.Wait()
}

func TestWebPushDeliverCommitsBeforeAsyncSend(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
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

func TestWebPushDeliverSynchronouslyRequiresPushAcceptance(t *testing.T) {
	t.Parallel()

	event := Event{Type: "test", Audience: Audience{TenantID: 41, Scope: ScopeTenant}, Title: "Test"}

	t.Run("without VAPID configuration", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.vapid = VAPIDConfig{}
		require.ErrorIs(t, channel.DeliverSynchronously(context.Background(), event), ErrNoWebPushSubscribers)
	})

	t.Run("without matching subscriptions", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		db, mock := mockTenantTx(t)
		channel.db = db

		require.ErrorIs(t, channel.DeliverSynchronously(context.Background(), event), ErrNoWebPushSubscribers)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestWebPushDeliverSynchronouslyPreservesCallerDeadline(t *testing.T) {
	t.Parallel()

	channel := testChannel(&fakePushRepo{staff: []*iot.PushSubscription{
		testSub(1, 41, "https://fcm.googleapis.com/device"),
	}}, &fakeSender{})
	db, mock := mockTenantTx(t)
	channel.db = db

	deadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	var sendDeadline time.Time
	channel.sender.(*fakeSender).sendFn = func(sendCtx context.Context) {
		var ok bool
		sendDeadline, ok = sendCtx.Deadline()
		require.True(t, ok, "synchronous sends must inherit the scheduler deadline")
	}

	require.NoError(t, channel.DeliverSynchronously(ctx, Event{
		Type:     "test",
		Audience: Audience{TenantID: 41, Scope: ScopeTenant},
		Title:    "Test",
	}))
	assert.WithinDuration(t, deadline, sendDeadline, 100*time.Millisecond)
	require.NoError(t, mock.ExpectationsWereMet())
}

// findForStaffAccounts is the batch path's finder. Kept next to the other
// finders on the same double so both delivery paths see one fake repository.
func (f *fakePushRepo) FindForStaffAccounts(_ context.Context, accountIDs []int64) ([]*iot.PushSubscription, error) {
	if f.staffAccountsErr != nil {
		return nil, f.staffAccountsErr
	}
	f.mu.Lock()
	f.staffAccountsAsked = append(f.staffAccountsAsked, accountIDs...)
	f.mu.Unlock()
	var out []*iot.PushSubscription
	for _, accountID := range accountIDs {
		out = append(out, f.staffAccounts[accountID]...)
	}
	return out, nil
}

func (f *fakePushRepo) FindForSchoolAccounts(_ context.Context, accountIDs []int64) ([]*iot.PushSubscription, error) {
	var out []*iot.PushSubscription
	for _, accountID := range accountIDs {
		out = append(out, f.schoolAccounts[accountID]...)
	}
	return out, nil
}

// mockTenantTx primes a sqlmock for exactly one tenant transaction.
func mockTenantTx(t *testing.T) (*bun.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	return db, mock
}

func staffEventFor(accountID int64, title string) Event {
	return Event{
		Type:     TypePickupUpcoming,
		Audience: Audience{TenantID: 41, Scope: ScopeStaff, StaffAccountIDs: []int64{accountID}},
		Priority: PriorityNormal,
		Title:    title,
	}
}

// DeliverBatch exists to resolve every recipient's devices in ONE transaction
// and still send each person their own payload. Both halves of that sentence
// are load-bearing: one transaction is the whole point, and a shared payload
// would hand one person another's message.
func TestWebPushDeliverBatchResolvesOnceAndSendsPerRecipient(t *testing.T) {
	t.Parallel()

	subA := testSub(1, 41, "https://fcm.googleapis.com/a")
	subA.AccountID = 11
	subB := testSub(2, 41, "https://fcm.googleapis.com/b")
	subB.AccountID = 12
	repo := &fakePushRepo{staffAccounts: map[int64][]*iot.PushSubscription{
		11: {subA},
		12: {subB},
	}}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)
	db, mock := mockTenantTx(t)
	channel.db = db

	require.NoError(t, channel.DeliverBatch(context.Background(), []Event{
		staffEventFor(11, "Abholung steht an"),
		staffEventFor(12, "Aktivität beginnt"),
	}))

	require.Eventually(t, func() bool {
		sender.mu.Lock()
		defer sender.mu.Unlock()
		return len(sender.sent) == 2
	}, time.Second, 10*time.Millisecond)

	assert.NoError(t, mock.ExpectationsWereMet(), "one transaction for the whole batch")
	assert.ElementsMatch(t, []int64{11, 12}, repo.staffAccountsAsked,
		"every recipient is resolved in the same read")

	byEndpoint := map[string]string{}
	sender.mu.Lock()
	for _, push := range sender.sent {
		byEndpoint[push.endpoint] = string(push.payload)
	}
	sender.mu.Unlock()
	assert.Contains(t, byEndpoint[subA.Endpoint], "Abholung steht an")
	assert.Contains(t, byEndpoint[subB.Endpoint], "Aktivität beginnt")
	assert.NotContains(t, byEndpoint[subA.Endpoint], "Aktivität beginnt",
		"each recipient gets their own payload")
}

func TestWebPushDeliverBatchEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("no events is a no-op", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		require.NoError(t, channel.DeliverBatch(context.Background(), nil))
	})

	t.Run("without VAPID keys the channel stays inert", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		channel.vapid = VAPIDConfig{}
		require.NoError(t, channel.DeliverBatch(context.Background(), []Event{staffEventFor(11, "x")}))
	})

	t.Run("a recipient without a device is skipped", func(t *testing.T) {
		withDevice := testSub(1, 41, "https://fcm.googleapis.com/a")
		withDevice.AccountID = 11
		repo := &fakePushRepo{staffAccounts: map[int64][]*iot.PushSubscription{
			11: {withDevice},
		}}
		sender := &fakeSender{}
		channel := testChannel(repo, sender)
		db, _ := mockTenantTx(t)
		channel.db = db

		require.NoError(t, channel.DeliverBatch(context.Background(), []Event{
			staffEventFor(11, "erreicht"),
			staffEventFor(12, "kein Gerät"),
		}))

		require.Eventually(t, func() bool {
			sender.mu.Lock()
			defer sender.mu.Unlock()
			return len(sender.sent) == 1
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("nobody has a device", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{}, &fakeSender{})
		db, _ := mockTenantTx(t)
		channel.db = db
		require.NoError(t, channel.DeliverBatch(context.Background(), []Event{staffEventFor(11, "x")}))
	})

	t.Run("a failing read is returned", func(t *testing.T) {
		channel := testChannel(&fakePushRepo{staffAccountsErr: errors.New("boom")}, &fakeSender{})
		db, _ := mockTenantTx(t)
		channel.db = db
		require.Error(t, channel.DeliverBatch(context.Background(), []Event{staffEventFor(11, "x")}))
	})
}

// Only staff-scoped events are grouped; the other scopes resolve their devices
// from the scope alone and go through the single path.
func TestWebPushDeliverBatchFallsBackForOtherScopes(t *testing.T) {
	t.Parallel()

	repo := &fakePushRepo{staff: []*iot.PushSubscription{
		testSub(1, 41, "https://fcm.googleapis.com/tenant"),
	}}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)
	db, _ := mockTenantTx(t)
	channel.db = db

	require.NoError(t, channel.DeliverBatch(context.Background(), []Event{{
		Type:     "test",
		Audience: Audience{TenantID: 41, Scope: ScopeTenant},
		Title:    "An alle",
	}}))

	require.Eventually(t, func() bool {
		sender.mu.Lock()
		defer sender.mu.Unlock()
		return len(sender.sent) == 1
	}, time.Second, 10*time.Millisecond)
}
