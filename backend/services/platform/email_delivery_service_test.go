package platform

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Test doubles. All three are behaviourally divergent from the shared doubles
// (error injection, call recording, per-key settings), so they stay local.
// -----------------------------------------------------------------------------

// deliveryOutboxRepo records ApplyDeliveryStatus calls and serves lookups from
// an in-memory index, so tests can drive correlation without SQL.
type deliveryOutboxRepo struct {
	stubOutboxRepo

	byMessageID         map[string]*platformModels.EmailOutbox
	byProviderMessageID map[string]*platformModels.EmailOutbox
	byID                map[int64]*platformModels.EmailOutbox

	applied     []platformModels.DeliveryTransition
	applyResult bool
	applyErr    error
	lookupErr   error
}

func (r *deliveryOutboxRepo) FindByMessageID(_ context.Context, id string) (*platformModels.EmailOutbox, error) {
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	return r.byMessageID[id], nil
}

func (r *deliveryOutboxRepo) FindByProviderMessageID(_ context.Context, id string) (*platformModels.EmailOutbox, error) {
	return r.byProviderMessageID[id], nil
}

func (r *deliveryOutboxRepo) FindByID(_ context.Context, id int64) (*platformModels.EmailOutbox, error) {
	row, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("email outbox row %d not found", id)
	}
	return row, nil
}

func (r *deliveryOutboxRepo) ApplyDeliveryStatus(_ context.Context, _ int64, t platformModels.DeliveryTransition) (bool, error) {
	if r.applyErr != nil {
		return false, r.applyErr
	}
	r.applied = append(r.applied, t)
	return r.applyResult, nil
}

// deliveryEventRepo records inserted events and can simulate the unique-index
// conflict that makes replays idempotent.
type deliveryEventRepo struct {
	created   []*platformModels.EmailDeliveryEvent
	seen      map[string]bool
	createErr error
}

func (r *deliveryEventRepo) CreateIfNew(_ context.Context, e *platformModels.EmailDeliveryEvent) (bool, error) {
	if r.createErr != nil {
		return false, r.createErr
	}
	if r.seen == nil {
		r.seen = map[string]bool{}
	}
	key := e.Provider + "|" + e.ProviderEventID
	if r.seen[key] {
		return false, nil
	}
	r.seen[key] = true
	r.created = append(r.created, e)
	return true, nil
}

func (r *deliveryEventRepo) ListByOutboxIDs(_ context.Context, _ []int64) ([]*platformModels.EmailDeliveryEvent, error) {
	return r.created, nil
}

func (r *deliveryEventRepo) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return int64(len(r.created)), nil
}

type recordingEnqueuer struct {
	requests []platformModels.OutboxEnqueueRequest
	err      error
}

func (e *recordingEnqueuer) EnqueueOutbox(_ context.Context, req platformModels.OutboxEnqueueRequest) error {
	if e.err != nil {
		return e.err
	}
	e.requests = append(e.requests, req)
	return nil
}

type stubSettings struct {
	bools   map[string]bool
	strings map[string]string
	ints    map[string]int
	err     error
}

func (s stubSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return s.bools[key], s.err
}

func (s stubSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	return s.strings[key], s.err
}

func (s stubSettings) ResolveIntForTenant(_ context.Context, _ int64, key string) (int, error) {
	return s.ints[key], s.err
}

type recordingMetrics struct {
	events   []string
	statuses []string
}

func (m *recordingMetrics) RecordDeliveryEvent(provider, eventType, result string) {
	m.events = append(m.events, provider+"/"+eventType+"/"+result)
}

func (m *recordingMetrics) RecordDeliveryStatus(status string) {
	m.statuses = append(m.statuses, status)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

const testMessageID = "ob.7.4321.abcd@mail.test"

func dispatchedRow(tenantID, id int64, kind string) *platformModels.EmailOutbox {
	messageID := testMessageID
	relatedType := platformModels.EmailRelatedTypeGuardianInvitation
	relatedID := int64(99)

	row := &platformModels.EmailOutbox{
		Kind:              kind,
		Status:            platformModels.EmailOutboxStatusSent,
		MessageID:         &messageID,
		RelatedEntityType: &relatedType,
		RelatedEntityID:   &relatedID,
	}
	row.ID = id
	row.SetTenantID(tenantID)
	return row
}

func repoWithRow(row *platformModels.EmailOutbox) *deliveryOutboxRepo {
	return &deliveryOutboxRepo{
		byMessageID: map[string]*platformModels.EmailOutbox{*row.MessageID: row},
		byID:        map[int64]*platformModels.EmailOutbox{row.ID: row},
		applyResult: true,
	}
}

func deliveryEvent(eventID, eventType, messageID string) DeliveryEvent {
	return DeliveryEvent{
		Provider:   "generic",
		EventID:    eventID,
		Type:       eventType,
		OccurredAt: time.Now(),
		MessageID:  messageID,
	}
}

// -----------------------------------------------------------------------------
// Ingest
// -----------------------------------------------------------------------------

func TestIngestEvent_AppliesDeliveredStatus(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := repoWithRow(row)
	eventRepo := &deliveryEventRepo{}
	metrics := &recordingMetrics{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, Metrics: metrics, DB: db,
	})

	outcome, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_1", platformModels.DeliveryEventDelivered, testMessageID))

	require.NoError(t, err)
	assert.Equal(t, IngestApplied, outcome)
	require.Len(t, outboxRepo.applied, 1)
	assert.Equal(t, platformModels.EmailDeliveryStatusDelivered, outboxRepo.applied[0].Status)
	require.Len(t, eventRepo.created, 1)
	assert.Equal(t, row.GetTenantID(), eventRepo.created[0].GetTenantID(),
		"the tenant must come from the resolved outbox row, never from the request")
	assert.Equal(t, []string{"generic/delivered/applied"}, metrics.events)
	assert.Equal(t, []string{platformModels.EmailDeliveryStatusDelivered}, metrics.statuses)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A replayed webhook body must change nothing the second time. The event
// table's unique index is the guard; the status write must not even be
// attempted again.
func TestIngestEvent_ReplayIsIdempotent(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)
	expectAdminTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := repoWithRow(row)
	eventRepo := &deliveryEventRepo{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, DB: db,
	})
	ev := deliveryEvent("evt_replayed", platformModels.DeliveryEventDelivered, testMessageID)

	first, err := svc.IngestEvent(context.Background(), ev)
	require.NoError(t, err)
	assert.Equal(t, IngestApplied, first)

	second, err := svc.IngestEvent(context.Background(), ev)
	require.NoError(t, err)
	assert.Equal(t, IngestDuplicate, second)

	assert.Len(t, eventRepo.created, 1, "the replay must not be stored twice")
	assert.Len(t, outboxRepo.applied, 1, "the replay must not re-apply the status")
	require.NoError(t, mock.ExpectationsWereMet())
}

// When the guarded UPDATE rejects a transition the event is still recorded —
// the history stays complete, only the summary column is monotone.
func TestIngestEvent_StaleTransitionStillStoresEvent(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := repoWithRow(row)
	outboxRepo.applyResult = false
	eventRepo := &deliveryEventRepo{}
	metrics := &recordingMetrics{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, Metrics: metrics, DB: db,
	})

	outcome, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_late", platformModels.DeliveryEventDeferred, testMessageID))

	require.NoError(t, err)
	assert.Equal(t, IngestStale, outcome)
	assert.Len(t, eventRepo.created, 1, "the audit trail keeps every event")
	assert.Empty(t, metrics.statuses, "a rejected transition must not count as applied")
	require.NoError(t, mock.ExpectationsWereMet())
}

// Events about mail we never sent are normal on a shared provider account.
// They must not be stored and must not error.
func TestIngestEvent_UnmatchedMessageIsNotStored(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	outboxRepo := &deliveryOutboxRepo{}
	eventRepo := &deliveryEventRepo{}
	metrics := &recordingMetrics{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, Metrics: metrics, DB: db,
	})

	outcome, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_stranger", platformModels.DeliveryEventDelivered, "ob.1.2.zzz@elsewhere.test"))

	require.NoError(t, err)
	assert.Equal(t, IngestUnmatched, outcome)
	assert.Empty(t, eventRepo.created)
	assert.Equal(t, []string{"generic/delivered/unmatched"}, metrics.events)
	require.NoError(t, mock.ExpectationsWereMet())
}

// The Message-ID local part is attacker-controlled. Resolving by it must
// re-verify the tenant against the stored row, or anyone who can reach the
// webhook could aim an event at an arbitrary message.
func TestIngestEvent_ForgedLocalPartTenantIsRejected(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := &deliveryOutboxRepo{
		byID:        map[int64]*platformModels.EmailOutbox{row.ID: row},
		applyResult: true,
	}
	eventRepo := &deliveryEventRepo{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, DB: db,
	})

	// Correct outbox id, WRONG tenant in the local part.
	outcome, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_forged", platformModels.DeliveryEventDelivered, "ob.999.4321.forged@mail.test"))

	require.NoError(t, err)
	assert.Equal(t, IngestUnmatched, outcome)
	assert.Empty(t, eventRepo.created)
	require.NoError(t, mock.ExpectationsWereMet())
}

// When the Message-ID was rewritten by a relay, the local part is the
// last-resort correlation path — and it works when tenant and id agree.
func TestIngestEvent_LocalPartFallbackResolves(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := &deliveryOutboxRepo{
		byID:        map[int64]*platformModels.EmailOutbox{row.ID: row},
		applyResult: true,
	}
	eventRepo := &deliveryEventRepo{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, DB: db,
	})

	outcome, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_fallback", platformModels.DeliveryEventDelivered, "<ob.7.4321.rewritten@relay.test>"))

	require.NoError(t, err)
	assert.Equal(t, IngestApplied, outcome)
	assert.Len(t, eventRepo.created, 1)
	require.NoError(t, mock.ExpectationsWereMet())
}

// An event type we do not understand must be dropped, never allowed to move
// the status.
func TestIngestEvent_UnknownEventTypeIsDropped(t *testing.T) {
	db, _, cleanup := newAdminTxDB(t)
	defer cleanup()

	outboxRepo := &deliveryOutboxRepo{}
	eventRepo := &deliveryEventRepo{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, DB: db,
	})

	outcome, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_future", "teleported", testMessageID))

	require.NoError(t, err)
	assert.Equal(t, IngestUnmatched, outcome)
	assert.Empty(t, eventRepo.created)
}

// A DB failure is the one case a provider SHOULD retry, so it must surface as
// an error rather than a swallowed 202.
func TestIngestEvent_DBFailurePropagates(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := repoWithRow(row)
	outboxRepo.applyErr = errors.New("database exploded")

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: &deliveryEventRepo{}, DB: db,
	})

	_, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_db", platformModels.DeliveryEventDelivered, testMessageID))

	require.Error(t, err)
}

func TestIngestBatch_TalliesEveryOutcome(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)
	expectAdminTx(mock)
	expectAdminTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	outboxRepo := repoWithRow(row)
	eventRepo := &deliveryEventRepo{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: outboxRepo, EventRepo: eventRepo, DB: db,
	})

	ev := deliveryEvent("evt_batch_1", platformModels.DeliveryEventDelivered, testMessageID)
	result, err := svc.IngestBatch(context.Background(), []DeliveryEvent{
		ev,
		ev, // replay
		deliveryEvent("evt_batch_2", platformModels.DeliveryEventDelivered, "ob.1.2.unknown@x.test"),
	})

	require.NoError(t, err)
	assert.Equal(t, 3, result.Accepted)
	assert.Equal(t, 1, result.Applied)
	assert.Equal(t, 1, result.Duplicate)
	assert.Equal(t, 1, result.Unmatched)
	require.NoError(t, mock.ExpectationsWereMet())
}

// -----------------------------------------------------------------------------
// Bounce notification
// -----------------------------------------------------------------------------

func expectTenantTx(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
}

func TestIngestEvent_HardBounceNotifiesConfiguredAddresses(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)
	expectTenantTx(mock)

	row := dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)
	enqueuer := &recordingEnqueuer{}

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: repoWithRow(row),
		EventRepo:  &deliveryEventRepo{},
		Enqueuer:   enqueuer,
		Settings: stubSettings{
			bools:   map[string]bool{configModels.KeyEmailDeliveryAlertsEnabled: true},
			strings: map[string]string{configModels.KeyEmailDeliveryAlertEmails: "buero@school.test, leitung@school.test"},
		},
		DB: db,
	})

	ev := deliveryEvent("evt_bounce", platformModels.DeliveryEventBounced, testMessageID)
	ev.BounceClass = platformModels.BounceClassHard
	ev.ReasonCode = "5.1.1"

	outcome, err := svc.IngestEvent(context.Background(), ev)
	require.NoError(t, err)
	assert.Equal(t, IngestApplied, outcome)

	require.Len(t, enqueuer.requests, 2, "one notice per configured address")
	assert.Equal(t, platformModels.EmailKindDeliveryFailureNotice, enqueuer.requests[0].Kind)
	// The idempotency key is what stops repeated bounce events from flooding
	// the office with the same notice.
	assert.Contains(t, enqueuer.requests[0].IdempotencyKey, "delivery-failure-4321-bounced")
	require.NoError(t, mock.ExpectationsWereMet())
}

// A temporary delay must NOT alert: the mail server is still working on it,
// and alerting here would train the office to ignore the alerts.
func TestIngestEvent_DeferredDoesNotNotify(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	enqueuer := &recordingEnqueuer{}
	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: repoWithRow(dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)),
		EventRepo:  &deliveryEventRepo{},
		Enqueuer:   enqueuer,
		Settings: stubSettings{
			bools:   map[string]bool{configModels.KeyEmailDeliveryAlertsEnabled: true},
			strings: map[string]string{configModels.KeyEmailDeliveryAlertEmails: "buero@school.test"},
		},
		DB: db,
	})

	_, err := svc.IngestEvent(context.Background(),
		deliveryEvent("evt_deferred", platformModels.DeliveryEventDeferred, testMessageID))

	require.NoError(t, err)
	assert.Empty(t, enqueuer.requests)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Loop guard: a bouncing admin address must not produce a bounce notice that
// bounces and produces another one.
func TestIngestEvent_BouncedNoticeDoesNotLoop(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	enqueuer := &recordingEnqueuer{}
	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: repoWithRow(dispatchedRow(7, 4321, platformModels.EmailKindDeliveryFailureNotice)),
		EventRepo:  &deliveryEventRepo{},
		Enqueuer:   enqueuer,
		Settings: stubSettings{
			bools:   map[string]bool{configModels.KeyEmailDeliveryAlertsEnabled: true},
			strings: map[string]string{configModels.KeyEmailDeliveryAlertEmails: "buero@school.test"},
		},
		DB: db,
	})

	ev := deliveryEvent("evt_notice_bounce", platformModels.DeliveryEventBounced, testMessageID)
	ev.BounceClass = platformModels.BounceClassHard

	_, err := svc.IngestEvent(context.Background(), ev)
	require.NoError(t, err)
	assert.Empty(t, enqueuer.requests, "a failed notice must not generate another notice")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIngestEvent_NoNotificationWhenAlertsDisabled(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)

	enqueuer := &recordingEnqueuer{}
	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: repoWithRow(dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)),
		EventRepo:  &deliveryEventRepo{},
		Enqueuer:   enqueuer,
		Settings: stubSettings{
			bools:   map[string]bool{configModels.KeyEmailDeliveryAlertsEnabled: false},
			strings: map[string]string{configModels.KeyEmailDeliveryAlertEmails: "buero@school.test"},
		},
		DB: db,
	})

	ev := deliveryEvent("evt_bounce_off", platformModels.DeliveryEventBounced, testMessageID)
	ev.BounceClass = platformModels.BounceClassHard

	_, err := svc.IngestEvent(context.Background(), ev)
	require.NoError(t, err)
	assert.Empty(t, enqueuer.requests)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A failing notification must never turn into a 500 that makes the provider
// retry the whole batch.
func TestIngestEvent_NotificationFailureDoesNotFailIngest(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectAdminTx(mock)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: repoWithRow(dispatchedRow(7, 4321, platformModels.EmailKindGuardianInvitation)),
		EventRepo:  &deliveryEventRepo{},
		Enqueuer:   &recordingEnqueuer{err: errors.New("outbox unavailable")},
		Settings: stubSettings{
			bools:   map[string]bool{configModels.KeyEmailDeliveryAlertsEnabled: true},
			strings: map[string]string{configModels.KeyEmailDeliveryAlertEmails: "buero@school.test"},
		},
		DB: db,
	})

	ev := deliveryEvent("evt_bounce_notifyfail", platformModels.DeliveryEventBounced, testMessageID)
	ev.BounceClass = platformModels.BounceClassHard

	outcome, err := svc.IngestEvent(context.Background(), ev)
	require.NoError(t, err, "a failed notification must not fail the ingest")
	assert.Equal(t, IngestApplied, outcome)
}

// -----------------------------------------------------------------------------
// Wiring + retention
// -----------------------------------------------------------------------------

func TestIngestEvent_UnwiredServiceErrors(t *testing.T) {
	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{})
	_, err := svc.IngestEvent(context.Background(), deliveryEvent("e", platformModels.DeliveryEventDelivered, testMessageID))
	require.Error(t, err)
}

func TestCleanupExpiredDeliveryEvents_UsesTenantRetentionSetting(t *testing.T) {
	db, mock, cleanup := newAdminTxDB(t)
	defer cleanup()
	expectTenantTx(mock)

	eventRepo := &deliveryEventRepo{created: []*platformModels.EmailDeliveryEvent{{}, {}}}
	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{
		OutboxRepo: &deliveryOutboxRepo{},
		EventRepo:  eventRepo,
		Settings:   stubSettings{ints: map[string]int{configModels.KeyEmailDeliveryRetentionDays: 7}},
		DB:         db,
	})

	deleted, err := svc.CleanupExpiredDeliveryEvents(context.Background(), 7)
	require.NoError(t, err)
	assert.EqualValues(t, 2, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCleanupExpiredDeliveryEvents_UnwiredServiceErrors(t *testing.T) {
	svc := NewEmailDeliveryService(EmailDeliveryServiceConfig{})
	_, err := svc.CleanupExpiredDeliveryEvents(context.Background(), 7)
	require.Error(t, err)
}

// -----------------------------------------------------------------------------
// Pure helpers
// -----------------------------------------------------------------------------

func TestDeliveryDetail_CombinesAndBounds(t *testing.T) {
	assert.Equal(t, "5.1.1 User unknown", deliveryDetail(DeliveryEvent{ReasonCode: "5.1.1", Reason: "User unknown"}))
	assert.Equal(t, "5.1.1", deliveryDetail(DeliveryEvent{ReasonCode: "5.1.1"}))
	assert.Empty(t, deliveryDetail(DeliveryEvent{}))

	long := deliveryDetail(DeliveryEvent{Reason: string(make([]byte, 900))})
	assert.LessOrEqual(t, len(long), 512, "the provider text must stay bounded")
}

func TestSplitEmails(t *testing.T) {
	assert.Equal(t, []string{"a@x.test", "b@x.test"}, splitEmails(" a@x.test , b@x.test ,, "))
	assert.Empty(t, splitEmails("   "))
}

func TestDeliveryFailureRenderer(t *testing.T) {
	render := NewDeliveryFailureRenderer(EmailConfig{})

	row := &platformModels.EmailOutbox{
		Kind: platformModels.EmailKindDeliveryFailureNotice,
		Payload: map[string]any{
			DeliveryNoticePayloadRecipient:    "buero@school.test",
			DeliveryNoticePayloadFailedTo:     "eltern@example.test",
			DeliveryNoticePayloadFailedStatus: platformModels.EmailDeliveryStatusBounced,
			DeliveryNoticePayloadFailedKind:   platformModels.EmailKindGuardianInvitation,
			DeliveryNoticePayloadReasonCode:   "5.1.1",
		},
	}

	msg, err := render(context.Background(), row)
	require.NoError(t, err)
	assert.Equal(t, "buero@school.test", msg.To.Address)
	assert.Equal(t, "delivery-failure-notice.html", msg.Template)

	content, ok := msg.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "endgültig nicht zustellbar", content["StatusLabel"])
	assert.Equal(t, "Einladung zum Eltern-Portal", content["KindLabel"])

	// A payload without a recipient is a programming error, not a mail.
	_, err = render(context.Background(), &platformModels.EmailOutbox{Kind: "x", Payload: map[string]any{}})
	require.Error(t, err)
}

func TestDeliveryFailureRenderer_FallsBackForUnknownLabels(t *testing.T) {
	render := NewDeliveryFailureRenderer(EmailConfig{})

	msg, err := render(context.Background(), &platformModels.EmailOutbox{
		Kind: platformModels.EmailKindDeliveryFailureNotice,
		Payload: map[string]any{
			DeliveryNoticePayloadRecipient:    "buero@school.test",
			DeliveryNoticePayloadFailedStatus: "something-new",
			DeliveryNoticePayloadFailedKind:   "some_future_kind",
		},
	})

	require.NoError(t, err)
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "nicht zustellbar", content["StatusLabel"])
	assert.Equal(t, "E-Mail", content["KindLabel"])
}
