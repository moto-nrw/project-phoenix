// The push a decided request sends to the submitting guardian (#1671), driven
// against a real thread so the consent read and the dispatch run inside the
// tenant transaction they need. The pure copy/eligibility rules live in
// decision_notification_internal_test.go.
package parentmessaging_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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

func TestEmitChildEvent_PushesDecisionToSubmittingGuardian(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
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

	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, staffDecision(chain.AccountID, 301))

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

// revokedBeforePush answers the child's guardian list truthfully once — for the
// pill's transaction — and empty afterwards, which is exactly what the push
// transaction sees when the guardian's parent_portal.access is withdrawn in
// between the two.
type revokedBeforePush struct {
	usersModels.ParentMessageThreadRepository
	reads int
}

func (r *revokedBeforePush) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*usersModels.MessageableGuardian, error) {
	r.reads++
	if r.reads > 1 {
		return nil, nil
	}
	return r.ParentMessageThreadRepository.ListGuardiansForStudent(ctx, studentID)
}

// The push runs in its own transaction, so it asks the access question again
// instead of trusting the answer the pill's (already committed) transaction got.
func TestEmitChildEvent_DecisionPushRechecksChildAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	threadRepo := &revokedBeforePush{ParentMessageThreadRepository: repos.ParentMessageThread}
	notifier := &capturingNotifier{}
	emitter := newMockEmitter(t, db, threadRepo, repos.ParentMessage,
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
		WithDecisionNotifications(notifier, decisionPreferences{optedIn: true})

	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, staffDecision(chain.AccountID, 309))

	assert.Empty(t, notifier.events,
		"an account the parent APIs now hide must not be told about the child on a lock screen")
	assert.Equal(t, 2, threadRepo.reads,
		"the push must read the access itself, not inherit the pill's verdict")
	_, msgs := threadPills(t, db, repos, chain)
	assert.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
		"the staff timeline still gets its closing pill")
}

func TestEmitChildEvent_DecisionPushRespectsConsentAndPillShape(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	t.Run("a guardian who did not opt in is not pushed at", func(t *testing.T) {
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		notifier := &capturingNotifier{}
		emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default()).
			WithDecisionNotifications(notifier, decisionPreferences{optedIn: false})

		emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, staffDecision(chain.AccountID, 302))

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

// A push is an extra, never a precondition: whatever the notification stack
// answers, the pill is already committed and the parent sees it in the app.
func TestEmitChildEvent_DecisionPushFailuresAreNotFatal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)

	emitWith := func(t *testing.T, notifier *capturingNotifier, prefs decisionPreferences, refID int64) string {
		t.Helper()
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
			&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), logger).
			WithDecisionNotifications(notifier, prefs)

		emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, staffDecision(chain.AccountID, refID))

		_, msgs := threadPills(t, db, repos, chain)
		require.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
			"the in-app pill survives every push outcome")
		return logs.String()
	}

	t.Run("an unavailable consent store is logged, not raised", func(t *testing.T) {
		logs := emitWith(t, &capturingNotifier{}, decisionPreferences{err: errors.New("preferences unavailable")}, 305)
		assert.Contains(t, logs, "request decision push failed")
	})

	t.Run("a school with notifications switched off is not a failure", func(t *testing.T) {
		notifier := &capturingNotifier{err: notifications.ErrDisabled}
		logs := emitWith(t, notifier, decisionPreferences{optedIn: true}, 306)
		assert.NotContains(t, logs, "request decision push failed",
			"a deliberately closed gate must not read as an error in the logs")
	})

	t.Run("a push outside the school's delivery window is not a failure", func(t *testing.T) {
		notifier := &capturingNotifier{err: notifications.ErrOutsideActiveWindow}
		logs := emitWith(t, notifier, decisionPreferences{optedIn: true}, 307)
		assert.NotContains(t, logs, "request decision push failed")
	})

	t.Run("a genuine dispatch failure is logged", func(t *testing.T) {
		notifier := &capturingNotifier{err: errors.New("push service unreachable")}
		logs := emitWith(t, notifier, decisionPreferences{optedIn: true}, 308)
		assert.Contains(t, logs, "request decision push failed")
	})
}

func TestWithDecisionNotificationsOnNilEmitter(t *testing.T) {
	t.Parallel()
	var emitter *parentmessaging.Emitter
	assert.Nil(t, emitter.WithDecisionNotifications(&capturingNotifier{}, decisionPreferences{}),
		"a partially-wired factory must not panic while adding optional dependencies")
}
