package notifications

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	webpush "github.com/SherClockHolmes/webpush-go"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// fakePushRepo implements the subscription lookups the channel needs; the
// embedded interface panics on anything else (nothing else may be called).
type fakePushRepo struct {
	deliveryModels.PushSubscriptionRepository
	staff              []*deliveryModels.PushSubscription
	admins             []*deliveryModels.PushSubscription
	guardians          map[int64][]*deliveryModels.PushSubscription
	deleted            []any
	deleteAttempts     []*deliveryModels.PushSubscription
	keepExpired        bool
	staffAccounts      map[int64][]*deliveryModels.PushSubscription
	schoolAccounts     map[int64][]*deliveryModels.PushSubscription
	staffAccountsAsked []int64
	staffAccountsErr   error
	deleteErr          error
	// guardiansStudentIDs records the children the guardian lookup was scoped
	// to, and hiddenStudentID is the child whose access the school has revoked.
	guardiansStudentIDs []int64
	hiddenStudentID     int64
	mu                  sync.Mutex
}

func (f *fakePushRepo) FindForTenantStaff(context.Context) ([]*deliveryModels.PushSubscription, error) {
	return f.staff, nil
}

func (f *fakePushRepo) FindForTenantAdmins(context.Context) ([]*deliveryModels.PushSubscription, error) {
	return f.admins, nil
}

func (f *fakePushRepo) FindForGuardians(_ context.Context, accountIDs []int64, studentIDs []int64) ([]*deliveryModels.PushSubscription, error) {
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
	var subscriptions []*deliveryModels.PushSubscription
	for _, accountID := range accountIDs {
		subscriptions = append(subscriptions, f.guardians[accountID]...)
	}
	return subscriptions, nil
}

func (f *fakePushRepo) DeleteExpiredIfUnchanged(_ context.Context, sub *deliveryModels.PushSubscription) (bool, error) {
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

type fakePushOutbox struct {
	intents       []PushIntent
	inTransaction []bool
	err           error
}

func (f *fakePushOutbox) EnqueuePush(ctx context.Context, intent PushIntent) (bool, error) {
	_, active := tenant.TransactionFromContext(ctx)
	f.inTransaction = append(f.inTransaction, active)
	if f.err != nil {
		return false, f.err
	}
	f.intents = append(f.intents, intent)
	return true, nil
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

func testSub(id, tenantID int64, endpoint string) *deliveryModels.PushSubscription {
	sub := &deliveryModels.PushSubscription{
		AccountID: 7,
		Portal:    deliveryModels.PushPortalStaff,
		Endpoint:  endpoint,
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
	}
	sub.ID = id
	sub.TenantID = tenantID
	return sub
}

func testChannel(repo *fakePushRepo, sender *fakeSender) *webPushChannel {
	channel := &webPushChannel{
		repo:      repo,
		vapid:     testVAPID(),
		sender:    sender,
		logger:    slog.Default(),
		sendSlots: make(chan struct{}, maxConcurrentPushSends),
		outbox:    &fakePushOutbox{},
	}
	channel.SetTenantRuntime(newMockTenantRuntime(nil, nil))
	return channel
}

func TestWebPushDeliverSnapshotsGDPRSafeIntent(t *testing.T) {
	t.Parallel()

	repo := &fakePushRepo{staff: []*deliveryModels.PushSubscription{testSub(1, 41, "https://fcm.googleapis.com/a")}}
	sender := &fakeSender{}
	channel := testChannel(repo, sender)
	outbox := channel.outbox.(*fakePushOutbox)

	event := Event{
		Type:           "reminders_due",
		IdempotencyKey: "reminder:1",
		RelatedType:    "reminder",
		RelatedID:      1,
		Audience:       Audience{TenantID: 41, Scope: ScopeTenant},
		Priority:       PriorityHigh,
		Title:          "3 Erinnerungen",
		Body:           "Es stehen Abholungen an.",
		DeepLink:       "/reminders",
		Data:           map[string]string{"student_name": "MUST-NEVER-LEAVE-THE-APP"},
	}

	require.NoError(t, channel.Deliver(context.Background(), event))
	require.Len(t, outbox.intents, 1)
	intent := outbox.intents[0]
	assert.Equal(t, "reminder:1:subscription:1", intent.IdempotencyKey)
	assert.Equal(t, event.Title, intent.Title)
	assert.Equal(t, event.DeepLink, intent.DeepLink)
	assert.Equal(t, event.RelatedType, intent.RelatedType)
	assert.Equal(t, event.RelatedID, intent.RelatedID)
	assert.Equal(t, ScopeTenant, intent.AudienceScope)
	assert.Empty(t, sender.sent, "durable delivery must not perform network I/O in the producer")
	encoded, err := json.Marshal(intent)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "MUST-NEVER-LEAVE-THE-APP")
}

func TestWebPushResolveSubscriptionsScopes(t *testing.T) {
	t.Parallel()

	staff := []*deliveryModels.PushSubscription{testSub(1, 41, "https://fcm.googleapis.com/s")}
	admins := []*deliveryModels.PushSubscription{testSub(2, 41, "https://fcm.googleapis.com/a")}
	guardian := testSub(3, 41, "https://fcm.googleapis.com/g")
	guardian.Portal = deliveryModels.PushPortalParent
	repo := &fakePushRepo{
		staff:     staff,
		admins:    admins,
		guardians: map[int64][]*deliveryModels.PushSubscription{99: {guardian}},
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
	assert.Equal(t, []*deliveryModels.PushSubscription{guardian}, got)

	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{
		TenantID:           41,
		Scope:              ScopeGuardian,
		GuardianAccountIDs: []int64{99},
	}})
	require.NoError(t, err)
	assert.Equal(t, []*deliveryModels.PushSubscription{guardian}, got)

	// A child-scoped audience carries the child into the device lookup, which is
	// where parent_portal.access is answered for the transaction that sends.
	got, err = channel.resolveEventSubscriptions(context.Background(), Event{Audience: Audience{
		TenantID:           41,
		Scope:              ScopeGuardian,
		GuardianAccountIDs: []int64{99},
		StudentIDs:         []int64{55},
	}})
	require.NoError(t, err)
	assert.Equal(t, []*deliveryModels.PushSubscription{guardian}, got)
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
	assert.Equal(t, []*deliveryModels.PushSubscription{guardian}, got)
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

func TestWebPushHTTPClientRejectsRedirects(t *testing.T) {
	t.Parallel()

	client := newWebPushHTTPClient()
	req, err := http.NewRequest(http.MethodGet, "https://fcm.googleapis.com/next", nil)
	require.NoError(t, err)
	assert.ErrorIs(t, client.CheckRedirect(req, nil), http.ErrUseLastResponse)
	assert.Equal(t, pushSendTimeout, client.Timeout)
}

func TestWebPushDeliverEnqueuesInsideTenantTransaction(t *testing.T) {
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

	sender := &fakeSender{}
	repo := &fakePushRepo{staff: []*deliveryModels.PushSubscription{
		testSub(1, 41, "https://fcm.googleapis.com/device"),
	}}
	channel := testChannel(repo, sender)
	channel.db = db
	channel.SetTenantRuntime(newMockTenantRuntime(t, db))

	require.NoError(t, channel.Deliver(context.Background(), Event{
		Type: "test", IdempotencyKey: "test:1",
		Audience: Audience{TenantID: 41, Scope: ScopeTenant}, Priority: PriorityNormal, Title: "Test",
	}))
	outbox := channel.outbox.(*fakePushOutbox)
	require.Equal(t, []bool{true}, outbox.inTransaction)
	require.Len(t, outbox.intents, 1)
	assert.Empty(t, sender.sent)
	require.NoError(t, mock.ExpectationsWereMet())
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
		channel.SetTenantRuntime(newMockTenantRuntime(t, db))

		require.ErrorIs(t, channel.DeliverSynchronously(context.Background(), event), ErrNoWebPushSubscribers)
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestWebPushDeliverSynchronouslyPreservesCallerDeadline(t *testing.T) {
	t.Parallel()

	channel := testChannel(&fakePushRepo{staff: []*deliveryModels.PushSubscription{
		testSub(1, 41, "https://fcm.googleapis.com/device"),
	}}, &fakeSender{})
	db, mock := mockTenantTx(t)
	channel.db = db
	channel.SetTenantRuntime(newMockTenantRuntime(t, db))

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
func (f *fakePushRepo) FindForStaffAccounts(_ context.Context, accountIDs []int64) ([]*deliveryModels.PushSubscription, error) {
	if f.staffAccountsErr != nil {
		return nil, f.staffAccountsErr
	}
	f.mu.Lock()
	f.staffAccountsAsked = append(f.staffAccountsAsked, accountIDs...)
	f.mu.Unlock()
	var out []*deliveryModels.PushSubscription
	for _, accountID := range accountIDs {
		out = append(out, f.staffAccounts[accountID]...)
	}
	return out, nil
}

func (f *fakePushRepo) FindForSchoolAccounts(_ context.Context, accountIDs []int64) ([]*deliveryModels.PushSubscription, error) {
	var out []*deliveryModels.PushSubscription
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
