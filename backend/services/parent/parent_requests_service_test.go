package parent_test

// Integration tests for the parent-side structured-request paths
// (parent_messaging_service.go): CreateChildRequest and WithdrawChildRequest end
// to end against the real test DB. These were previously unexercised. The focus
// is the durable, security-relevant behavior:
//   - a request persists only the CANONICAL payload (no client-supplied junk),
//   - submitting/withdrawing a structured request requires
//     parent_portal.request.submit — NOT the plain parent_portal.notes.write that
//     chat needs (sending a message must not grant authority to mutate master data),
//   - withdraw stays available after the school disables messaging.
//
// Reuses harness from parent_messaging_read_service_test.go (same package):
// buildReadService (wires the guardian-profile repo so the request carries the
// guardian's real name), stubSettings, captureBroadcaster.

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func openRequestMessage(t *testing.T, msgs []*usersModels.ParentMessage) *usersModels.ParentMessage {
	t.Helper()
	for _, m := range msgs {
		if m.Kind == usersModels.ParentMessageKindRequest {
			return m
		}
	}
	t.Fatalf("no request message found among %d messages", len(msgs))
	return nil
}

func parentEvent(t *testing.T, msgs []*usersModels.ParentMessage) *usersModels.ParentMessage {
	t.Helper()
	for _, m := range msgs {
		if m.Kind == usersModels.ParentMessageKindEvent {
			return m
		}
	}
	t.Fatalf("no event message found among %d messages", len(msgs))
	return nil
}

// TestCreateChildRequest_PersistsCanonicalPayloadAndOpensRequest is the guardian
// "Anfrage stellen" happy path: a care-schedule change request is created as an
// OPEN request message carrying a diff for the card, and the persisted payload is
// the CANONICAL form — duplicate weekday entries collapsed (last value per aspect
// wins) and unknown keys dropped — so a direct API client cannot store ignored
// junk that every later thread load echoes back.
func TestCreateChildRequest_PersistsCanonicalPayloadAndOpensRequest(t *testing.T) {
	svc, _, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	payload := map[string]any{
		"weekdays": []any{
			map[string]any{"weekday": 1, "arrival": "08:00"},
			map[string]any{"weekday": 1, "arrival": "09:00"}, // duplicate Monday, last wins
			map[string]any{"weekday": 1, "pickup": "15:00"},
		},
		"junk": "ignored",
	}

	view, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule, payload)
	require.NoError(t, err)
	require.NotNil(t, view)

	req := openRequestMessage(t, view.Messages)
	assert.Equal(t, usersModels.ParentMessageRequestStatusOpen, req.RequestStatus)
	assert.Equal(t, usersModels.ParentMessageRequestCareSchedule, req.RequestType)
	assert.Equal(t, usersModels.ParentMessageSenderGuardian, req.SenderKind)
	assert.Equal(t, "Sabine Schneider", req.SenderName, "the guardian's real name is denormalized onto the request")

	// The PERSISTED payload is canonical: one Monday entry (arrival 09:00 + pickup
	// 15:00), no "junk" key.
	reloaded, err := repos.ParentMessage.FindByID(tenant.WithTenantID(context.Background(), 1), req.ID)
	require.NoError(t, err)
	_, hasJunk := reloaded.Payload["junk"]
	assert.False(t, hasJunk, "unknown keys are dropped before persisting")
	weekdays, ok := reloaded.Payload["weekdays"].([]any)
	require.True(t, ok)
	require.Len(t, weekdays, 1, "duplicate weekday entries collapse to one")
	monday, ok := weekdays[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "09:00", monday["arrival"], "last arrival value wins")
	assert.Equal(t, "15:00", monday["pickup"])
}

// TestCreateChildRequest_RequiresRequestSubmitPermission is the core security
// separation: a change request OVERWRITES the child's care schedule once staff
// confirm, so it requires parent_portal.request.submit — NOT the
// parent_portal.notes.write that plain chat needs. A guardian who can chat must
// not, by that fact alone, be able to submit a master-data-mutating request.
func TestCreateChildRequest_RequiresRequestSubmitPermission(t *testing.T) {
	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Grant chat (notes.write) but explicitly NOT request.submit.
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true, "parent_portal.notes.write": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	ctx := context.Background()
	// Chat still works under notes.write.
	_, err = svc.PostChildMessage(ctx, chain.AccountID, chain.StudentID, "Hallo OGS")
	require.NoError(t, err, "notes.write still allows chat")

	// But a structured request is refused without request.submit.
	_, err = svc.CreateChildRequest(ctx, chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied,
		"submitting a master-data-mutating request requires request.submit, not just notes.write")
}

// TestCreateChildRequest_UnknownTypeRejected: the request type must be a
// registered handler (the allowlist derives from the same registry the apply
// uses), so an unknown type is ErrInvalidParentRequestType before any insert.
func TestCreateChildRequest_UnknownTypeRejected(t *testing.T) {
	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		"some.unknown.type", map[string]any{})
	require.ErrorIs(t, err, parentService.ErrInvalidParentRequestType)
}

// TestCreateChildRequest_InvalidPayloadRejected: a known type with a malformed
// payload (weekday out of the 1-5 range) fails validation/canonicalization and is
// rejected, never persisted.
func TestCreateChildRequest_InvalidPayloadRejected(t *testing.T) {
	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 9, "mode": "bus"}}})
	require.Error(t, err, "an invalid weekday must be rejected")
}

// TestCreateChildRequest_NotLinkedDenied: a guardian may not submit a request for
// a child they do not guardian — the single most security-critical invariant,
// enforced through the shared permitted-child resolve.
func TestCreateChildRequest_NotLinkedDenied(t *testing.T) {
	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.CreateChildRequest(context.Background(), chain.AccountID, other.ID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// TestCreateChildRequest_DisabledFeatureRefused: creating a request needs
// messaging enabled (like sending a message), so with the feature off it is
// ErrNotesDisabled.
func TestCreateChildRequest_DisabledFeatureRefused(t *testing.T) {
	svc, _, db, _ := buildReadService(t, false) // messaging OFF
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.ErrorIs(t, err, parentService.ErrNotesDisabled)
}

// TestWithdrawChildRequest_FlipsToWithdrawnAndPostsEvent: a guardian withdraws
// their own open request; the request flips to 'zurueckgezogen' and a
// parent-visible "Anfrage zurückgezogen" system event is appended.
func TestWithdrawChildRequest_FlipsToWithdrawnAndPostsEvent(t *testing.T) {
	svc, _, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	created, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.NoError(t, err)
	reqID := openRequestMessage(t, created.Messages).ID

	view, err := svc.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
	require.NoError(t, err)

	event := parentEvent(t, view.Messages)
	assert.Equal(t, "Anfrage zurückgezogen", event.Body)
	assert.Equal(t, usersModels.ParentMessageRequestStatusWithdrawn, event.RequestStatus)
	assert.Equal(t, usersModels.ParentMessageSenderSystem, event.SenderKind)

	reloaded, err := repos.ParentMessage.FindByID(tenant.WithTenantID(context.Background(), 1), reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusWithdrawn, reloaded.RequestStatus)
}

// TestWithdrawChildRequest_NotOpenRejected: a request can only be withdrawn while
// open. Withdrawing it a second time is ErrParentRequestNotOpen (re-read under the
// FOR UPDATE lock), so a withdraw cannot race a staff decision into double action.
func TestWithdrawChildRequest_NotOpenRejected(t *testing.T) {
	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	created, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.NoError(t, err)
	reqID := openRequestMessage(t, created.Messages).ID

	_, err = svc.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
	require.NoError(t, err)

	_, err = svc.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
	require.ErrorIs(t, err, parentService.ErrParentRequestNotOpen)
}

// TestWithdrawChildRequest_NotFound: a request id that is not a request on the
// guardian's thread is ErrParentRequestNotFound, not a 500.
func TestWithdrawChildRequest_NotFound(t *testing.T) {
	svc, _, db, _ := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Open a thread so the lookup gets past the thread resolve, then withdraw a
	// non-existent request id.
	_, err := svc.PostChildMessage(context.Background(), chain.AccountID, chain.StudentID, "Hallo")
	require.NoError(t, err)

	_, err = svc.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, 999999999)
	require.ErrorIs(t, err, parentService.ErrParentRequestNotFound)
}

// TestWithdrawChildRequest_WorksWhenDisabled is the documented contract that
// withdraw (unlike create) deliberately skips the enabled-check: a parent must be
// able to wind down an outstanding request even after the school turns messaging
// OFF, instead of leaving it frozen open forever.
func TestWithdrawChildRequest_WorksWhenDisabled(t *testing.T) {
	svc, bc, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	created, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.NoError(t, err)
	reqID := openRequestMessage(t, created.Messages).ID

	// Rebuild the service with messaging OFF, same DB/repos.
	disabled := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		Settings:            stubSettings{sickEnabled: true, notesEnabled: false},
		Broadcaster:         bc,
		MessageThreadRepo:   repos.ParentMessageThread,
		MessageRepo:         repos.ParentMessage,
		MessageReadRepo:     repos.ParentMessageRead,
		DB:                  db,
		Logger:              slog.Default(),
	})

	_, err = disabled.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
	require.NoError(t, err, "withdraw must stay available even with messaging disabled")

	reloaded, err := repos.ParentMessage.FindByID(tenant.WithTenantID(context.Background(), 1), reqID)
	require.NoError(t, err)
	assert.Equal(t, usersModels.ParentMessageRequestStatusWithdrawn, reloaded.RequestStatus)
}

// --- decision error / rollback contract (fault injection) ----------------

// failingParentMessageRepo wraps the real repo and fails one chosen write so the
// parent request paths' error handling is exercised: a write that fails
// mid-create/withdraw MUST surface as an error (so the tenant tx rolls back),
// never reporting success on a half-written request.
type failingParentMessageRepo struct {
	usersModels.ParentMessageRepository
	failCreate bool
	failUpdate bool
}

func (r failingParentMessageRepo) Create(ctx context.Context, m *usersModels.ParentMessage) error {
	if r.failCreate {
		return errInjectedParent
	}
	return r.ParentMessageRepository.Create(ctx, m)
}

func (r failingParentMessageRepo) Update(ctx context.Context, m *usersModels.ParentMessage) error {
	if r.failUpdate {
		return errInjectedParent
	}
	return r.ParentMessageRepository.Update(ctx, m)
}

type errInjectedParentT string

func (e errInjectedParentT) Error() string { return string(e) }

const errInjectedParent = errInjectedParentT("injected repository failure")

func buildParentMsgServiceWithRepo(t *testing.T, db *bun.DB, repos *repositories.Factory, msgRepo usersModels.ParentMessageRepository) parentService.Service {
	t.Helper()
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		Settings:            stubSettings{sickEnabled: true, notesEnabled: true},
		Broadcaster:         &captureBroadcaster{},
		MessageThreadRepo:   repos.ParentMessageThread,
		MessageRepo:         msgRepo,
		MessageReadRepo:     repos.ParentMessageRead,
		DB:                  db,
		Logger:              slog.Default(),
	})
}

// TestCreateChildRequest_RepoFailurePropagates: if persisting the request
// message fails, CreateChildRequest must return an error (→ tenant tx rollback),
// never report a created request that was never stored.
func TestCreateChildRequest_RepoFailurePropagates(t *testing.T) {
	_, _, db, repos := buildReadService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	svc := buildParentMsgServiceWithRepo(t, db, repos,
		failingParentMessageRepo{ParentMessageRepository: repos.ParentMessage, failCreate: true})

	_, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
		usersModels.ParentMessageRequestCareSchedule,
		map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
	require.Error(t, err, "a failed request-message persist must fail the create")
}

// TestWithdrawChildRequest_RepoFailuresPropagate: both the status flip and the
// withdrawal event are inside the withdraw tx; a failure in either must surface
// as an error, distinct from the not-found/not-open guards.
func TestWithdrawChildRequest_RepoFailuresPropagate(t *testing.T) {
	createOpen := func(t *testing.T) (*bun.DB, *repositories.Factory, testpkg.ParentChain, int64) {
		t.Helper()
		svc, _, db, repos := buildReadService(t, true)
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
		created, err := svc.CreateChildRequest(context.Background(), chain.AccountID, chain.StudentID,
			usersModels.ParentMessageRequestCareSchedule,
			map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}})
		require.NoError(t, err)
		return db, repos, chain, openRequestMessage(t, created.Messages).ID
	}

	t.Run("status update failure", func(t *testing.T) {
		db, repos, chain, reqID := createOpen(t)
		svc := buildParentMsgServiceWithRepo(t, db, repos,
			failingParentMessageRepo{ParentMessageRepository: repos.ParentMessage, failUpdate: true})
		_, err := svc.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
		require.Error(t, err)
		require.NotErrorIs(t, err, parentService.ErrParentRequestNotOpen)
		require.NotErrorIs(t, err, parentService.ErrParentRequestNotFound)
	})

	t.Run("withdrawal event failure", func(t *testing.T) {
		db, repos, chain, reqID := createOpen(t)
		svc := buildParentMsgServiceWithRepo(t, db, repos,
			failingParentMessageRepo{ParentMessageRepository: repos.ParentMessage, failCreate: true})
		_, err := svc.WithdrawChildRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
		require.Error(t, err)
	})
}
