// The push a decided request sends to the submitting guardian (#1671), driven
// against a real thread so the consent read and the dispatch run inside the
// tenant transaction they need. The pure copy/eligibility rules live in
// decision_notification_internal_test.go.
package messaging_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	parentmessaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/application/parentmessaging"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	"github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type capturingNotifier struct {
	events []communication.NotificationEvent
	err    error
}

func (n *capturingNotifier) Notify(_ context.Context, event communication.NotificationEvent) error {
	if n.err != nil {
		return n.err
	}
	n.events = append(n.events, event)
	return nil
}

// decisionPreferences answers the consent question the emitter asks before it
// pushes: optedIn decides the audience, err simulates an unavailable store.
type decisionPreferences struct {
	communication.NotificationPreferences
	optedIn bool
	err     error
}

func (p decisionPreferences) FilterOptedIn(_ context.Context, _ string, accountIDs []int64) ([]int64, error) {
	if p.err != nil {
		return nil, p.err
	}
	if !p.optedIn {
		return nil, nil
	}
	return accountIDs, nil
}

func staffDecision(accountID int64, refID int64) eventContract.ChildEvent {
	return eventContract.ChildEvent{
		EventType:      messaging.ParentMessageEventRequestStatus,
		ActorKind:      messaging.ParentMessageSenderStaff,
		ActorAccountID: accountID,
		Body:           "Anfrage bestätigt",
		RequestType:    messaging.ParentMessageRequestCareSchedule,
		RequestStatus:  messaging.ParentMessageRequestStatusDone,
		RefTable:       "schedule.care_schedule_change_requests",
		RefID:          i64(refID),
	}
}

func enqueueDecision(
	t *testing.T,
	db *bun.DB,
	emitter eventContract.Emitter,
	tenantID, studentID, accountID int64,
	ev eventContract.ChildEvent,
) error {
	t.Helper()
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, db))
	return tenant.WithTenantTx(ctx, db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return emitter.EnqueueRequestDecision(ctx, tenantID, studentID, accountID, ev)
	})
}

func TestEmitChildEvent_PushesDecisionToSubmittingGuardian(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", "en").
		Where("account_id = ?", chain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	notifier := &capturingNotifier{}
	emitter := newMockEmitter(t, db, messaging.NewThreadRepository(reposMessages.ParentMessageThread), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default(),
		api.NewCommunicationRequestDecisionNotifier(notifier.Notify, decisionPreferences{optedIn: true}))

	ev := staffDecision(chain.AccountID, 301)
	require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev))
	_ = emitter.EmitChildEvent(context.Background(), chain.TenantID, chain.StudentID, chain.AccountID, ev)

	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Equal(t, communication.TypeParentRequestDecided, event.Type)
	assert.Equal(t, "Request approved", event.Title)
	assert.Contains(t, event.Body, "care schedule")
	assert.Equal(t, communication.ScopeGuardian, event.Audience.Scope)
	assert.Equal(t, chain.TenantID, event.Audience.TenantID)
	assert.Equal(t, []int64{chain.AccountID}, event.Audience.GuardianAccountIDs)
	// The deep link points at the child, never at a payload that names them.
	assert.Contains(t, event.DeepLink, "/children/")
	assert.NotContains(t, event.Body, "Kind")

	_, msgs := threadPills(t, db, reposMessages, chain)
	assert.Equal(t, 1, countEventType(msgs, messaging.ParentMessageEventRequestStatus),
		"the pill and the push are the same event, not two code paths")
}

// revokedBeforePush denies child access to both the delivery and pill paths.
type revokedBeforePush struct {
	messaging.ParentMessageThreadRepository
	reads int
}

func (r *revokedBeforePush) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*messaging.MessageableGuardian, error) {
	r.reads++
	return nil, nil
}

func TestRequestDecisionEnqueueAndPillBothCheckChildAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	threadRepo := &revokedBeforePush{ParentMessageThreadRepository: messaging.NewThreadRepository(reposMessages.ParentMessageThread)}
	notifier := &capturingNotifier{}
	emitter := newMockEmitter(t, db, threadRepo, messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default(),
		api.NewCommunicationRequestDecisionNotifier(notifier.Notify, decisionPreferences{optedIn: true}))

	ev := staffDecision(chain.AccountID, 309)
	require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev))
	_ = emitter.EmitChildEvent(context.Background(), chain.TenantID, chain.StudentID, chain.AccountID, ev)

	assert.Empty(t, notifier.events,
		"an account the parent APIs now hide must not be told about the child on a lock screen")
	assert.Equal(t, 2, threadRepo.reads,
		"delivery enqueue and the detached pill each enforce current access")
	_, msgs := threadPills(t, db, reposMessages, chain)
	assert.Zero(t, countEventType(msgs, messaging.ParentMessageEventRequestStatus))
}

func TestEmitChildEvent_DecisionPushRespectsConsentAndPillShape(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))

	t.Run("a guardian who did not opt in is not pushed at", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		notifier := &capturingNotifier{}
		emitter := newMockEmitter(t, db, messaging.NewThreadRepository(reposMessages.ParentMessageThread), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default(),
			api.NewCommunicationRequestDecisionNotifier(notifier.Notify, decisionPreferences{optedIn: false}))

		ev := staffDecision(chain.AccountID, 302)
		require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev))
		_ = emitter.EmitChildEvent(context.Background(), chain.TenantID, chain.StudentID, chain.AccountID, ev)

		assert.Empty(t, notifier.events)
		_, msgs := threadPills(t, db, reposMessages, chain)
		assert.Equal(t, 1, countEventType(msgs, messaging.ParentMessageEventRequestStatus),
			"consent governs the push, not the in-app pill")
	})

	t.Run("a guardian withdrawing their own request is not pushed back at", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		notifier := &capturingNotifier{}
		emitter := newMockEmitter(t, db, messaging.NewThreadRepository(reposMessages.ParentMessageThread), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default(),
			api.NewCommunicationRequestDecisionNotifier(notifier.Notify, decisionPreferences{optedIn: true}))

		withdrawn := staffDecision(chain.AccountID, 303)
		withdrawn.ActorKind = messaging.ParentMessageSenderGuardian
		withdrawn.RequestStatus = messaging.ParentMessageRequestStatusWithdrawn
		require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, withdrawn))
		_ = emitter.EmitChildEvent(context.Background(), chain.TenantID, chain.StudentID, chain.AccountID, withdrawn)

		assert.Empty(t, notifier.events)
	})

	t.Run("an emitter without notification wiring stays a no-op", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		emitter := newMockEmitter(t, db, messaging.NewThreadRepository(reposMessages.ParentMessageThread), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default())

		_ = emitter.EmitChildEvent(context.Background(), chain.TenantID, chain.StudentID, chain.AccountID, staffDecision(chain.AccountID, 304))

		_, msgs := threadPills(t, db, reposMessages, chain)
		assert.Equal(t, 1, countEventType(msgs, messaging.ParentMessageEventRequestStatus),
			"the pill must still be written when no notifier is configured")
	})
}

func TestRequestDecisionEnqueueFailuresAbortTheUnitOfWork(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))

	emitWith := func(t *testing.T, notifier *capturingNotifier, prefs decisionPreferences, refID int64) (int, error) {
		t.Helper()
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		emitter := newMockEmitter(t, db, messaging.NewThreadRepository(reposMessages.ParentMessageThread), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)),
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default(),
			api.NewCommunicationRequestDecisionNotifier(notifier.Notify, prefs))

		ev := staffDecision(chain.AccountID, refID)
		err := enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev)
		if err == nil {
			_ = emitter.EmitChildEvent(context.Background(), chain.TenantID, chain.StudentID, chain.AccountID, ev)
		}

		_, msgs := threadPills(t, db, reposMessages, chain)
		return countEventType(msgs, messaging.ParentMessageEventRequestStatus), err
	}

	t.Run("an unavailable consent store aborts", func(t *testing.T) {
		pills, err := emitWith(t, &capturingNotifier{}, decisionPreferences{err: errors.New("preferences unavailable")}, 305)
		assert.ErrorContains(t, err, "preferences unavailable")
		assert.Zero(t, pills)
	})

	t.Run("a school with notifications switched off remains a closed gate", func(t *testing.T) {
		notifier := &capturingNotifier{err: devices.ErrNotificationsDisabled}
		pills, err := emitWith(t, notifier, decisionPreferences{optedIn: true}, 306)
		assert.NoError(t, err)
		assert.Equal(t, 1, pills)
	})

	t.Run("a push outside the school's delivery window is not a failure", func(t *testing.T) {
		notifier := &capturingNotifier{err: devices.ErrOutsideNotificationWindow}
		pills, err := emitWith(t, notifier, decisionPreferences{optedIn: true}, 307)
		assert.NoError(t, err)
		assert.Equal(t, 1, pills)
	})

	t.Run("a genuine enqueue failure aborts", func(t *testing.T) {
		notifier := &capturingNotifier{err: errors.New("push service unreachable")}
		pills, err := emitWith(t, notifier, decisionPreferences{optedIn: true}, 308)
		assert.ErrorContains(t, err, "push service unreachable")
		assert.Zero(t, pills)
	})
}

func TestDecisionDeliveryOnNilEmitter(t *testing.T) {
	t.Parallel()
	var emitter *parentmessaging.Emitter
	assert.NoError(t, emitter.EnqueueRequestDecision(context.Background(), 1, 2, 3, staffDecision(3, 310)),
		"a partially-wired factory must not panic on its optional decision delivery")
}
