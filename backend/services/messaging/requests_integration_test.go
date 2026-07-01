package messaging_test

// Integration tests for the staff-side request DECISION paths
// (services/messaging/requests.go): ConfirmRequest and RejectRequest end to end
// against the real test DB, the real schedule/student/user-context services, and
// a capturing broadcaster — exactly as the HTTP handlers drive them.
//
// The existing service_test.go fixture deliberately leaves Students/UserContext
// unwired (its tests never reach the apply path), which is why ConfirmRequest's
// apply was previously untested. newDecideFixture wires the FULL stack so a
// confirm actually applies the child's care schedule, and the assertions pin the
// durable outcome (status, applied schedule, decision event), not just the call
// returning nil. Helpers reused from service_test.go (same package): stubSettings,
// captureBroadcaster, adminCtx, claimsCtx, newPersons, createOpenRequest.

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/messaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// decideFixture is the fully-wired staff messaging stack (Students + UserContext
// + schedule services present, unlike service_test.go's newFixture) so the
// confirm apply path runs for real. sf is kept so a test can read back the
// applied schedules through the same services production uses.
type decideFixture struct {
	db           *bun.DB
	svc          messaging.Service
	bc           *captureBroadcaster
	chain        testpkg.ParentChain
	staffAccount int64
	sf           *services.Factory
}

func newDecideFixture(t *testing.T, enabled bool) *decideFixture {
	return newDecideFixtureRepo(t, enabled, nil)
}

// failingMessageRepo wraps the real ParentMessageRepository and fails one
// chosen write, so the decision-path error/rollback contract can be exercised:
// a write that fails mid-confirm/reject MUST surface as an error (so
// TenantTxMiddleware rolls the whole operation back) rather than silently
// committing a half-applied change. Reads (FindByID / FindByIDForUpdate /
// ListByThread) delegate to the embedded real repo.
type failingMessageRepo struct {
	usersModels.ParentMessageRepository
	failCreate bool
	failUpdate bool
}

func (r failingMessageRepo) Create(ctx context.Context, m *usersModels.ParentMessage) error {
	if r.failCreate {
		return errInjected
	}
	return r.ParentMessageRepository.Create(ctx, m)
}

func (r failingMessageRepo) Update(ctx context.Context, m *usersModels.ParentMessage) error {
	if r.failUpdate {
		return errInjected
	}
	return r.ParentMessageRepository.Update(ctx, m)
}

var errInjected = errInjectedT("injected repository failure")

type errInjectedT string

func (e errInjectedT) Error() string { return string(e) }

// newDecideFixtureRepo wires the full decide stack; wrapMessageRepo, when set,
// replaces the message repo with a fault-injecting wrapper (the rest of the
// stack — including request seeding via the real repo factory — is unaffected).
func newDecideFixtureRepo(t *testing.T, enabled bool, wrapMessageRepo func(usersModels.ParentMessageRepository) usersModels.ParentMessageRepository) *decideFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	sf, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	bc := &captureBroadcaster{}
	msgRepo := usersModels.ParentMessageRepository(repos.ParentMessage)
	if wrapMessageRepo != nil {
		msgRepo = wrapMessageRepo(repos.ParentMessage)
	}
	svc := messaging.NewService(messaging.Config{
		ThreadRepo:  repos.ParentMessageThread,
		MessageRepo: msgRepo,
		ReadRepo:    repos.ParentMessageRead,
		Persons:     sf.Users,
		Students:    sf.Students,
		Arrival:     sf.ArrivalSchedule,
		Pickup:      sf.PickupSchedule,
		UserContext: sf.UserContext,
		Settings:    stubSettings{messagingEnabled: enabled},
		Broadcaster: bc,
		DB:          db,
		Logger:      slog.Default(),
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Clara", "Confirm")
	t.Cleanup(func() { testpkg.CleanupStaffFixtures(t, db, staff.ID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, staffAccount.ID) })
	t.Cleanup(func() { testpkg.CleanupParentMessagingForAccount(t, db, staffAccount.ID) })
	return &decideFixture{db: db, svc: svc, bc: bc, chain: chain, staffAccount: staffAccount.ID, sf: sf}
}

// createOpenCareRequest inserts a still-open guardian care-schedule request with
// the given weekday payload entries straight through the repo, so the decision
// paths have something decidable with a richer payload than createOpenRequest.
func createOpenCareRequest(t *testing.T, db *bun.DB, chain testpkg.ParentChain, threadID int64, weekdays []any) int64 {
	t.Helper()
	req := &usersModels.ParentMessage{
		ThreadID:        threadID,
		StudentID:       chain.StudentID,
		SenderAccountID: chain.AccountID,
		SenderKind:      usersModels.ParentMessageSenderGuardian,
		SenderName:      "Sabine Schneider",
		Body:            messaging.RequestBody(usersModels.ParentMessageRequestCareSchedule),
		Kind:            usersModels.ParentMessageKindRequest,
		RequestType:     usersModels.ParentMessageRequestCareSchedule,
		RequestStatus:   usersModels.ParentMessageRequestStatusOpen,
		Payload:         map[string]any{"weekdays": weekdays},
	}
	req.SetTenantID(chain.TenantID)
	require.NoError(t, repositories.NewFactory(db).ParentMessage.Create(tenant.WithTenantID(context.Background(), 1), req))
	return req.ID
}

// findRequestEvent returns the single confirm/reject decision event in the
// message list (Kind=event, EventType="request_status").
func findDecisionEvent(t *testing.T, msgs []*usersModels.ParentMessage) *usersModels.ParentMessage {
	t.Helper()
	for _, m := range msgs {
		if m.Kind == usersModels.ParentMessageKindEvent && m.EventType == "request_status" {
			return m
		}
	}
	t.Fatalf("no request_status decision event found among %d messages", len(msgs))
	return nil
}

func findRequestMessage(t *testing.T, msgs []*usersModels.ParentMessage, reqID int64) *usersModels.ParentMessage {
	t.Helper()
	for _, m := range msgs {
		if m.ID == reqID {
			return m
		}
	}
	t.Fatalf("request message %d not found among %d messages", reqID, len(msgs))
	return nil
}

// TestConfirmRequest_AppliesCareScheduleAndPostsEvent is the staff "Bestätigen"
// happy path. Confirming a still-open care-schedule request must DURABLY apply
// the child's permanent weekly plan (departure mode + arrival + pickup for the
// requested weekday), flip the request to 'erledigt' with the actor/timestamp
// stamped, post a parent-visible "übernommen" decision event into the thread, and
// wake the guardian with a message broadcast. This is the core value of the
// feature; before this test the apply path was unexercised because the existing
// fixture left the schedule services unwired.
func TestConfirmRequest_AppliesCareScheduleAndPostsEvent(t *testing.T) {
	f := newDecideFixture(t, true)
	ctx := adminCtx(f.staffAccount)

	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	reqID := createOpenCareRequest(t, f.db, f.chain, thread.ThreadID, []any{
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	})
	f.bc.parent = nil // isolate the confirm's own broadcast

	msgs, diffs, err := f.svc.ConfirmRequest(ctx, reqID)
	require.NoError(t, err)

	// The decision is recorded in the thread: the request flips to done and a
	// "Betreuungszeiten übernommen" event is appended.
	reqMsg := findRequestMessage(t, msgs, reqID)
	assert.Equal(t, usersModels.ParentMessageRequestStatusDone, reqMsg.RequestStatus)
	event := findDecisionEvent(t, msgs)
	assert.Equal(t, "Anfrage bestätigt, Betreuungszeiten übernommen", event.Body)
	assert.Equal(t, usersModels.ParentMessageRequestStatusDone, event.RequestStatus)
	assert.Equal(t, usersModels.ParentMessageSenderSystem, event.SenderKind)
	// The confirmed request is closed, so it carries no open-request diff anymore.
	assert.Empty(t, diffs[reqID], "a confirmed request loses its open diff")

	// The request row is persisted as done with the acting staffer + timestamp.
	reloaded, err := repositories.NewFactory(f.db).ParentMessage.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusDone, reloaded.RequestStatus)
	require.NotNil(t, reloaded.AppliedAt, "AppliedAt is stamped on confirm")
	require.NotNil(t, reloaded.AppliedBy)
	assert.Equal(t, f.staffAccount, *reloaded.AppliedBy, "the confirming staffer is recorded")

	// The child's permanent care plan was actually changed (this is the point of a
	// confirm — not just a status flip).
	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, arrivals, 1)
	assert.Equal(t, 1, arrivals[0].Weekday)
	assert.Equal(t, "08:00", arrivals[0].ExpectedArrival.Format("15:04"))
	pickups, err := f.sf.PickupSchedule.GetStudentPickupSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, pickups, 1)
	assert.Equal(t, "16:00", pickups[0].PickupTime.Format("15:04"))
	student, err := f.sf.Users.GetStudentByID(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.DeparturePickup, student.DepartureDays["mon"])

	// The guardian is woken so their open chat refreshes to the decided thread.
	assert.GreaterOrEqual(t, f.bc.countOf(realtime.EventParentMessage), 1, "confirm wakes the guardian")
}

// TestConfirmRequest_SecondConfirmIsInvalidStatus pins the race/idempotency guard:
// once a request is applied, a second confirm must NOT apply the schedule change a
// second time — lockOpenRequest re-reads the row under FOR UPDATE and rejects a
// non-open status with ErrInvalidRequestStatus.
func TestConfirmRequest_SecondConfirmIsInvalidStatus(t *testing.T) {
	f := newDecideFixture(t, true)
	ctx := adminCtx(f.staffAccount)

	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	reqID := createOpenCareRequest(t, f.db, f.chain, thread.ThreadID, []any{
		map[string]any{"weekday": 2, "arrival": "07:45"},
	})

	_, _, err = f.svc.ConfirmRequest(ctx, reqID)
	require.NoError(t, err)

	_, _, err = f.svc.ConfirmRequest(ctx, reqID)
	require.ErrorIs(t, err, messaging.ErrInvalidRequestStatus, "a decided request cannot be confirmed twice")

	// And a reject racing the same already-applied request must not flip it either.
	_, _, err = f.svc.RejectRequest(ctx, reqID, "zu spät")
	require.ErrorIs(t, err, messaging.ErrInvalidRequestStatus, "an applied request cannot then be rejected")
}

// TestConfirmRequest_DisabledFeatureRefused: confirm is gated on
// operations.parent_notes_enabled (it applies a change and notifies a recipient),
// so with the feature off it returns ErrMessagingDisabled BEFORE applying.
func TestConfirmRequest_DisabledFeatureRefused(t *testing.T) {
	f := newDecideFixture(t, false)
	ctx := adminCtx(f.staffAccount)

	// Seed thread + open request directly (the confirm gate is what we exercise).
	repos := repositories.NewFactory(f.db)
	th, err := repos.ParentMessageThread.GetOrCreate(tenant.WithTenantID(context.Background(), 1), f.chain.TenantID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	reqID := createOpenCareRequest(t, f.db, f.chain, th.ID, []any{map[string]any{"weekday": 1, "arrival": "08:00"}})

	_, _, err = f.svc.ConfirmRequest(ctx, reqID)
	require.ErrorIs(t, err, messaging.ErrMessagingDisabled)

	// The request stays open and nothing was applied.
	reloaded, err := repos.ParentMessage.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusOpen, reloaded.RequestStatus)
	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, arrivals, "a refused confirm applies nothing")
}

// TestConfirmRequest_NotFound: confirming a non-existent id, or a plain
// (non-request) message id, is ErrRequestNotFound — never a 500 or a wrong apply.
func TestConfirmRequest_NotFound(t *testing.T) {
	f := newDecideFixture(t, true)
	ctx := adminCtx(f.staffAccount)

	_, _, err := f.svc.ConfirmRequest(ctx, 999999999)
	require.ErrorIs(t, err, messaging.ErrRequestNotFound, "unknown id is not found")

	// A plain guardian message is not a decidable request.
	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	createGuardianMessage(t, f.db, f.chain, thread.ThreadID, "Nur eine Nachricht")
	msgs, err := f.svc.GetThread(ctx, thread.ThreadID)
	require.NoError(t, err)
	var plainID int64
	for _, m := range msgs.Messages {
		if m.Kind == usersModels.ParentMessageKindMessage && m.SenderKind == usersModels.ParentMessageSenderGuardian {
			plainID = m.ID
		}
	}
	require.Positive(t, plainID)
	_, _, err = f.svc.ConfirmRequest(ctx, plainID)
	require.ErrorIs(t, err, messaging.ErrRequestNotFound, "a plain message is not a request")
}

// --- RejectRequest ------------------------------------------------------
// Reject runs on the basic fixture (newFixture from service_test.go) because it
// never reaches the apply path — it only flips status, posts the decision event,
// and rebuilds diffs, all of which the basic fixture's repos/schedule services
// already support.

// TestConfirmRequest_RevokedGuardianRefused pins the documented asymmetry: a
// confirm writes a parent-visible event and applies a change for a recipient, so
// it stays gated on the guardian still being linked with parent_portal.access.
// Confirming a request whose guardian was unlinked is ErrGuardianAccessRevoked,
// and nothing is applied — staff must reject (wind down) such a request instead.
func TestConfirmRequest_RevokedGuardianRefused(t *testing.T) {
	f := newDecideFixture(t, true)
	ctx := adminCtx(f.staffAccount)

	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	reqID := createOpenCareRequest(t, f.db, f.chain, thread.ThreadID, []any{
		map[string]any{"weekday": 1, "arrival": "08:00"},
	})

	// Revoke the guardian's portal access.
	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, f.chain.TenantID, f.chain.StudentID, f.chain.GuardianProfileID)
	require.NoError(t, err)

	_, _, err = f.svc.ConfirmRequest(ctx, reqID)
	require.ErrorIs(t, err, messaging.ErrGuardianAccessRevoked)

	// The request stays open and nothing was applied.
	reloaded, err := repositories.NewFactory(f.db).ParentMessage.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusOpen, reloaded.RequestStatus)
	arrivals, err := f.sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, arrivals, "a refused confirm applies nothing")
}

// TestRejectRequest_RevokedGuardianStillSucceeds is the other half of the
// asymmetry: reject deliberately skips the guardian-link gate so staff can wind
// down an outstanding request after the guardian is unlinked, instead of leaving
// it frozen open. The broadcast is suppressed (no leak to a revoked account) but
// the reject itself succeeds.
func TestRejectRequest_RevokedGuardianStillSucceeds(t *testing.T) {
	f := newFixture(t, true)
	ctx := adminCtx(f.staffAccount)

	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	reqID := createOpenRequest(t, f.db, f.chain, thread.ThreadID)

	_, err = f.db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, f.chain.TenantID, f.chain.StudentID, f.chain.GuardianProfileID)
	require.NoError(t, err)
	f.bc.parent = nil

	msgs, _, err := f.svc.RejectRequest(ctx, reqID, "Anfrage nicht mehr nötig")
	require.NoError(t, err, "reject stays available after the guardian is unlinked")
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, findRequestMessage(t, msgs, reqID).RequestStatus)
	assert.Equal(t, 0, f.bc.countOf(realtime.EventParentMessage), "no SSE fan-out to a revoked guardian")
}

// TestRejectRequest_MarksRejectedWithoutApplying: rejecting flips the request to
// 'abgelehnt' with the reason, posts the "Anfrage abgelehnt: …" event, and must
// NOT touch the child's care schedule.
func TestRejectRequest_MarksRejectedWithoutApplying(t *testing.T) {
	f := newFixture(t, true)
	ctx := adminCtx(f.staffAccount)

	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	reqID := createOpenRequest(t, f.db, f.chain, thread.ThreadID)

	msgs, _, err := f.svc.RejectRequest(ctx, reqID, "Bitte mit Datum erneut einreichen")
	require.NoError(t, err)

	reqMsg := findRequestMessage(t, msgs, reqID)
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, reqMsg.RequestStatus)
	event := findDecisionEvent(t, msgs)
	assert.Equal(t, "Anfrage abgelehnt: Bitte mit Datum erneut einreichen", event.Body)
	assert.Equal(t, "Bitte mit Datum erneut einreichen", event.DecisionReason)

	reloaded, err := repositories.NewFactory(f.db).ParentMessage.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, reloaded.RequestStatus)
	assert.Equal(t, "Bitte mit Datum erneut einreichen", reloaded.DecisionReason)

	// No schedule change happened on a reject.
	sf, err := services.NewFactory(repositories.NewFactory(f.db), f.db, slog.Default())
	require.NoError(t, err)
	arrivals, err := sf.ArrivalSchedule.GetStudentArrivalSchedules(ctx, f.chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, arrivals, "rejecting must not apply the requested plan")
}

// TestRejectRequest_ReasonRequiredAndBounded pins the free-text guards: a blank
// reason is ErrRejectReasonRequired, an over-2000-rune reason is
// ErrRejectReasonTooLong, both BEFORE any state change.
func TestRejectRequest_ReasonRequiredAndBounded(t *testing.T) {
	f := newFixture(t, true)
	ctx := adminCtx(f.staffAccount)
	thread, err := f.svc.StartThread(ctx, f.chain.StudentID, f.chain.AccountID, "Hallo")
	require.NoError(t, err)
	reqID := createOpenRequest(t, f.db, f.chain, thread.ThreadID)

	_, _, err = f.svc.RejectRequest(ctx, reqID, "   \n\t ")
	require.ErrorIs(t, err, messaging.ErrRejectReasonRequired)

	_, _, err = f.svc.RejectRequest(ctx, reqID, strings.Repeat("x", 2001))
	require.ErrorIs(t, err, messaging.ErrRejectReasonTooLong)

	// The request is untouched after the rejected guards.
	reloaded, err := repositories.NewFactory(f.db).ParentMessage.FindByID(ctx, reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusOpen, reloaded.RequestStatus)
}

// TestRejectRequest_WorksWhenFeatureDisabled is the documented contract that
// reject (unlike confirm) deliberately skips the requireEnabled gate: staff must
// be able to wind down an outstanding request after an admin turns messaging OFF,
// instead of leaving it frozen in the open-request queue forever.
func TestRejectRequest_WorksWhenFeatureDisabled(t *testing.T) {
	f := newFixture(t, false) // feature OFF
	ctx := adminCtx(f.staffAccount)

	// Seed thread + open request directly (StartThread would be refused while off).
	repos := repositories.NewFactory(f.db)
	th, err := repos.ParentMessageThread.GetOrCreate(tenant.WithTenantID(context.Background(), 1), f.chain.TenantID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	reqID := createOpenRequest(t, f.db, f.chain, th.ID)

	msgs, _, err := f.svc.RejectRequest(ctx, reqID, "Schule eingestellt")
	require.NoError(t, err, "reject must stay available even with messaging disabled")
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, findRequestMessage(t, msgs, reqID).RequestStatus)
}

// --- decision error / rollback contract (fault injection) ----------------
// These pin the invariant the code comments stress most: a write that fails
// mid-decision MUST surface as an error so the request transaction rolls back,
// never committing a half-applied change or a decided-but-uneventful request.

// TestConfirmRequest_RepoUpdateFailurePropagates: if persisting the request's
// new 'erledigt' status fails after the schedule apply, ConfirmRequest must
// return a (non-guard) error so TenantTxMiddleware rolls the apply back, rather
// than reporting success on a request still stored as open.
func TestConfirmRequest_RepoUpdateFailurePropagates(t *testing.T) {
	f := newDecideFixtureRepo(t, true, func(r usersModels.ParentMessageRepository) usersModels.ParentMessageRepository {
		return failingMessageRepo{ParentMessageRepository: r, failUpdate: true}
	})
	ctx := adminCtx(f.staffAccount)
	repos := repositories.NewFactory(f.db)
	th, err := repos.ParentMessageThread.GetOrCreate(tenant.WithTenantID(context.Background(), 1), f.chain.TenantID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	reqID := createOpenCareRequest(t, f.db, f.chain, th.ID, []any{map[string]any{"weekday": 1, "arrival": "08:00"}})

	_, _, err = f.svc.ConfirmRequest(ctx, reqID)
	require.Error(t, err, "a failed status persist must surface as an error (→ rollback)")
	require.NotErrorIs(t, err, messaging.ErrInvalidRequestStatus, "this is a real failure, not the decided-status guard")
}

// TestConfirmRequest_EventCreateFailurePropagates: if appending the decision
// event fails, the confirm must fail too (no silent apply without the
// parent-visible "übernommen" event).
func TestConfirmRequest_EventCreateFailurePropagates(t *testing.T) {
	f := newDecideFixtureRepo(t, true, func(r usersModels.ParentMessageRepository) usersModels.ParentMessageRepository {
		return failingMessageRepo{ParentMessageRepository: r, failCreate: true}
	})
	ctx := adminCtx(f.staffAccount)
	repos := repositories.NewFactory(f.db)
	th, err := repos.ParentMessageThread.GetOrCreate(tenant.WithTenantID(context.Background(), 1), f.chain.TenantID, f.chain.StudentID, f.chain.AccountID)
	require.NoError(t, err)
	reqID := createOpenCareRequest(t, f.db, f.chain, th.ID, []any{map[string]any{"weekday": 1, "arrival": "08:00"}})

	_, _, err = f.svc.ConfirmRequest(ctx, reqID)
	require.Error(t, err, "a failed decision-event append must fail the confirm")
}

// TestRejectRequest_RepoFailuresPropagate: both the status persist and the
// decision-event append are inside the reject; a failure in either must surface
// as an error so the reject is not reported done on a still-open request.
func TestRejectRequest_RepoFailuresPropagate(t *testing.T) {
	seed := func(f *decideFixture) int64 {
		repos := repositories.NewFactory(f.db)
		th, err := repos.ParentMessageThread.GetOrCreate(tenant.WithTenantID(context.Background(), 1), f.chain.TenantID, f.chain.StudentID, f.chain.AccountID)
		require.NoError(t, err)
		return createOpenRequest(t, f.db, f.chain, th.ID)
	}

	t.Run("status update failure", func(t *testing.T) {
		f := newDecideFixtureRepo(t, true, func(r usersModels.ParentMessageRepository) usersModels.ParentMessageRepository {
			return failingMessageRepo{ParentMessageRepository: r, failUpdate: true}
		})
		_, _, err := f.svc.RejectRequest(adminCtx(f.staffAccount), seed(f), "Bitte erneut")
		require.Error(t, err)
		require.NotErrorIs(t, err, messaging.ErrInvalidRequestStatus)
	})

	t.Run("event append failure", func(t *testing.T) {
		f := newDecideFixtureRepo(t, true, func(r usersModels.ParentMessageRepository) usersModels.ParentMessageRepository {
			return failingMessageRepo{ParentMessageRepository: r, failCreate: true}
		})
		_, _, err := f.svc.RejectRequest(adminCtx(f.staffAccount), seed(f), "Bitte erneut")
		require.Error(t, err)
	})
}
