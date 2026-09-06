// The push a decided request sends to the submitting guardian (#1671), driven
// against a real thread so the consent read and the dispatch run inside the
// tenant transaction they need. The pure copy/eligibility rules live in
// decision_notification_internal_test.go.
package parentmessaging_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

type capturingNotifier struct {
	events []notifications.Event
	err    error
}

func (n *capturingNotifier) Notify(_ context.Context, event notifications.Event) error {
	if n.err != nil {
		return n.err
	}
	n.events = append(n.events, event)
	return nil
}

// decisionPreferences answers the consent question the emitter asks before it
// pushes: optedIn decides the audience, err simulates an unavailable store.
type decisionPreferences struct {
	notifications.PreferenceService
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

func staffDecision(accountID int64, refID int64) parentmessaging.ChildEvent {
	return parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: accountID,
		Body:           "Anfrage bestätigt",
		RequestType:    usersModels.ParentMessageRequestCareSchedule,
		RequestStatus:  usersModels.ParentMessageRequestStatusDone,
		RefTable:       "schedule.care_schedule_change_requests",
		RefID:          i64(refID),
	}
}

func enqueueDecision(
	t *testing.T,
	db *bun.DB,
	emitter *parentmessaging.Emitter,
	tenantID, studentID, accountID int64,
	ev parentmessaging.ChildEvent,
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
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := db.NewUpdate().
		TableExpr("users.guardian_profiles").
		Set("portal_locale = ?", "en").
		Where("account_id = ?", chain.AccountID).
		Exec(context.Background())
	require.NoError(t, err)

	notifier := &capturingNotifier{}
	emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
		WithDecisionNotifications(notifier, decisionPreferences{optedIn: true})

	ev := staffDecision(chain.AccountID, 301)
	require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev))
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, ev)

	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Equal(t, notifications.TypeParentRequestDecided, event.Type)
	assert.Equal(t, "Request approved", event.Title)
	assert.Contains(t, event.Body, "care schedule")
	assert.Equal(t, notifications.ScopeGuardian, event.Audience.Scope)
	assert.Equal(t, chain.TenantID, event.Audience.TenantID)
	assert.Equal(t, []int64{chain.AccountID}, event.Audience.GuardianAccountIDs)
	// The deep link points at the child, never at a payload that names them.
	assert.Contains(t, event.DeepLink, "/children/")
	assert.NotContains(t, event.Body, "Kind")

	_, msgs := threadPills(t, db, repos, chain)
	assert.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
		"the pill and the push are the same event, not two code paths")
}

// revokedBeforePush denies child access to both the delivery and pill paths.
type revokedBeforePush struct {
	usersModels.ParentMessageThreadRepository
	reads int
}

func (r *revokedBeforePush) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error) {
	r.reads++
	return nil, nil
}

func TestRequestDecisionEnqueueAndPillBothCheckChildAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	threadRepo := &revokedBeforePush{ParentMessageThreadRepository: repos.ParentMessageThread}
	notifier := &capturingNotifier{}
	emitter := newMockEmitter(t, db, threadRepo, repos.ParentMessage,
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
		WithDecisionNotifications(notifier, decisionPreferences{optedIn: true})

	ev := staffDecision(chain.AccountID, 309)
	require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev))
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, ev)

	assert.Empty(t, notifier.events,
		"an account the parent APIs now hide must not be told about the child on a lock screen")
	assert.Equal(t, 2, threadRepo.reads,
		"delivery enqueue and the detached pill each enforce current access")
	_, msgs := threadPills(t, db, repos, chain)
	assert.Zero(t, countEventType(msgs, usersModels.ParentMessageEventRequestStatus))
}

func TestEmitChildEvent_DecisionPushRespectsConsentAndPillShape(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	t.Run("a guardian who did not opt in is not pushed at", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		notifier := &capturingNotifier{}
		emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
			WithDecisionNotifications(notifier, decisionPreferences{optedIn: false})

		ev := staffDecision(chain.AccountID, 302)
		require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev))
		emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, ev)

		assert.Empty(t, notifier.events)
		_, msgs := threadPills(t, db, repos, chain)
		assert.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
			"consent governs the push, not the in-app pill")
	})

	t.Run("a guardian withdrawing their own request is not pushed back at", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		notifier := &capturingNotifier{}
		emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
			WithDecisionNotifications(notifier, decisionPreferences{optedIn: true})

		withdrawn := staffDecision(chain.AccountID, 303)
		withdrawn.ActorKind = usersModels.ParentMessageSenderGuardian
		withdrawn.RequestStatus = usersModels.ParentMessageRequestStatusWithdrawn
		require.NoError(t, enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, withdrawn))
		emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, withdrawn)

		assert.Empty(t, notifier.events)
	})

	t.Run("an emitter without notification wiring stays a no-op", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default())

		emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, staffDecision(chain.AccountID, 304))

		_, msgs := threadPills(t, db, repos, chain)
		assert.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
			"the pill must still be written when no notifier is configured")
	})
}

func TestRequestDecisionEnqueueFailuresAbortTheUnitOfWork(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	emitWith := func(t *testing.T, notifier *capturingNotifier, prefs decisionPreferences, refID int64) (int, error) {
		t.Helper()
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
			WithDecisionNotifications(notifier, prefs)

		ev := staffDecision(chain.AccountID, refID)
		err := enqueueDecision(t, db, emitter, chain.TenantID, chain.StudentID, chain.AccountID, ev)
		if err == nil {
			emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, ev)
		}

		_, msgs := threadPills(t, db, repos, chain)
		return countEventType(msgs, usersModels.ParentMessageEventRequestStatus), err
	}

	t.Run("an unavailable consent store aborts", func(t *testing.T) {
		pills, err := emitWith(t, &capturingNotifier{}, decisionPreferences{err: errors.New("preferences unavailable")}, 305)
		assert.ErrorContains(t, err, "preferences unavailable")
		assert.Zero(t, pills)
	})

	t.Run("a school with notifications switched off remains a closed gate", func(t *testing.T) {
		notifier := &capturingNotifier{err: notifications.ErrDisabled}
		pills, err := emitWith(t, notifier, decisionPreferences{optedIn: true}, 306)
		assert.NoError(t, err)
		assert.Equal(t, 1, pills)
	})

	t.Run("a push outside the school's delivery window is not a failure", func(t *testing.T) {
		notifier := &capturingNotifier{err: notifications.ErrOutsideActiveWindow}
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

func TestWithDecisionNotificationsOnNilEmitter(t *testing.T) {
	t.Parallel()
	var emitter *parentmessaging.Emitter
	assert.Nil(t, emitter.WithDecisionNotifications(&capturingNotifier{}, decisionPreferences{}),
		"a partially-wired factory must not panic while adding optional dependencies")
}
