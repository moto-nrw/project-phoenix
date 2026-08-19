package parentmessaging_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/parentmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Real-DB integration tests for Emitter.EmitChildEvent (#1803). The unit tests
// in parentmessaging_test.go use fake repos and never exercise the tenant-tx
// write path; these drive the actual thread/pill writes so the no-orphan-thread
// invariant (a disabled school or a revoked guardian must never gain a thread)
// is verified end-to-end.

type toggleSettings struct{ enabled bool }

func (s *toggleSettings) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return s.enabled, nil
}

func i64(v int64) *int64 { return &v }

// threadPills loads the (student, guardian) thread and its messages, or nil when
// no thread exists. Runs inside the tenant tx so RLS admits the rows.
func threadPills(t *testing.T, db *bun.DB, repos *repositories.Factory, c testpkg.ParentChain) (*usersModels.ParentMessageThread, []*usersModels.ParentMessage) {
	t.Helper()
	var thread *usersModels.ParentMessageThread
	var msgs []*usersModels.ParentMessage
	err := tenant.WithTenantTx(context.Background(), db, c.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		th, ferr := repos.ParentMessageThread.FindByStudentGuardian(txCtx, c.StudentID, c.AccountID)
		if ferr != nil || th == nil {
			return ferr
		}
		thread = th
		ms, merr := repos.ParentMessage.ListByThread(txCtx, th.ID, 100)
		msgs = ms
		return merr
	})
	require.NoError(t, err)
	return thread, msgs
}

func countEventType(msgs []*usersModels.ParentMessage, eventType string) int {
	n := 0
	for _, m := range msgs {
		if m.EventType == eventType {
			n++
		}
	}
	return n
}

// TestEmitChildEvent_CreatesThreadThenReconcilesClose walks the full pill
// lifecycle against a real thread:
//  1. a guardian-triggered request_created pill lazily creates the thread;
//  2. a guardian self-service (sick_note) mirror advances the thread preview;
//  3. with messaging turned OFF, a staff terminal decision still reconciles the
//     CLOSING pill onto the existing thread (so the staff timeline stops showing
//     the request as actionable), without ever creating a new thread.
func TestEmitChildEvent_CreatesThreadThenReconcilesClose(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	settings := &toggleSettings{enabled: true}
	bc := testpkg.NewRecordingBroadcaster()
	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage, settings, bc, slog.Default())

	// 1) Guardian files a request → request_created pill, thread born.
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestCreated,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: chain.AccountID,
		Body:           "Anfrage gestellt",
		RequestType:    usersModels.ParentMessageRequestMasterData,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
		RefTable:       "users.student_data_change_requests",
		RefID:          i64(101),
	})
	thread, msgs := threadPills(t, db, repos, chain)
	require.NotNil(t, thread, "a filed request lazily creates the thread")
	require.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestCreated))

	// 2) Guardian self-service mirror pill advances the thread preview + wakes SSE.
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      "sick_note",
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: chain.AccountID,
		Body:           "Krankmeldung eingereicht",
	})
	_, msgs = threadPills(t, db, repos, chain)
	require.Len(t, msgs, 2, "the self-service mirror pill lands in the thread")

	// 3) Messaging OFF; a staff decision on the SAME request still writes the
	//    closing pill onto the existing thread (reconcile), no new thread.
	settings.enabled = false
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: chain.AccountID,
		Body:           "Anfrage bestätigt",
		RequestType:    usersModels.ParentMessageRequestMasterData,
		RequestStatus:  usersModels.ParentMessageRequestStatusDone,
		RefTable:       "users.student_data_change_requests",
		RefID:          i64(101),
	})
	_, msgs = threadPills(t, db, repos, chain)
	assert.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
		"the closing pill reconciles onto the existing thread even with messaging off")

	// 4) Messaging OFF terminal for a DIFFERENT request with no request_created
	//    pill to close is dropped — no dangling status pill appears.
	before := len(msgs)
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: chain.AccountID,
		Body:           "Anfrage abgelehnt",
		RequestType:    usersModels.ParentMessageRequestMasterData,
		RequestStatus:  usersModels.ParentMessageRequestStatusRejected,
		RefTable:       "users.student_data_change_requests",
		RefID:          i64(999),
	})
	_, msgs = threadPills(t, db, repos, chain)
	assert.Len(t, msgs, before, "a terminal event with no matching request_created pill adds nothing")
}

// TestEmitChildEvent_DisabledSchoolNeverBornsThread pins the core no-orphan
// invariant: with messaging disabled and no pre-existing thread, a staff
// terminal decision drops entirely rather than creating a thread via GetOrCreate.
func TestEmitChildEvent_DisabledSchoolNeverBornsThread(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		&toggleSettings{enabled: false}, testpkg.NewRecordingBroadcaster(), slog.Default())

	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: chain.AccountID,
		Body:           "Anfrage bestätigt",
		RequestType:    usersModels.ParentMessageRequestMasterData,
		RequestStatus:  usersModels.ParentMessageRequestStatusDone,
		RefTable:       "users.student_data_change_requests",
		RefID:          i64(202),
	})

	thread, _ := threadPills(t, db, repos, chain)
	assert.Nil(t, thread, "a disabled school must not gain a thread from a staff decision")
}

// TestEmitChildEvent_DisabledDropsNonTerminal covers the pre-tx fast-drop: with
// messaging off a NON-terminal pill (here a self-service mirror) is dropped
// before any thread work, so no thread is created.
func TestEmitChildEvent_DisabledDropsNonTerminal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		&toggleSettings{enabled: false}, testpkg.NewRecordingBroadcaster(), slog.Default())

	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      "sick_note",
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: chain.AccountID,
		Body:           "Krankmeldung",
	})

	thread, _ := threadPills(t, db, repos, chain)
	assert.Nil(t, thread, "a disabled school drops a non-terminal pill without creating a thread")
}

// TestEmitChildEvent_RevokedGuardianClosesWithoutWaking pins the revoked-access
// invariant: after a request_created pill exists, if the submitting guardian
// loses parent_portal.access, a staff terminal decision STILL writes the closing
// pill (so the staff timeline stops showing the request as actionable) but the
// guardian SSE fan-out is suppressed (broadcast guardian id 0) — a parent who
// can no longer read the child is never woken for it.
func TestEmitChildEvent_RevokedGuardianClosesWithoutWaking(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	bc := testpkg.NewRecordingBroadcaster()
	emitter := parentmessaging.NewEmitter(db, repos.ParentMessageThread, repos.ParentMessage,
		&toggleSettings{enabled: true}, bc, slog.Default())

	// A request_created pill exists (thread born while the guardian had access).
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestCreated,
		ActorKind:      usersModels.ParentMessageSenderGuardian,
		ActorAccountID: chain.AccountID,
		Body:           "Anfrage gestellt",
		RequestType:    usersModels.ParentMessageRequestMasterData,
		RequestStatus:  usersModels.ParentMessageRequestStatusOpen,
		RefTable:       "users.student_data_change_requests",
		RefID:          i64(303),
	})
	thread, _ := threadPills(t, db, repos, chain)
	require.NotNil(t, thread)

	// The guardian loses parent_portal.access after submitting.
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	bc.Reset()
	// Staff decides the request: the closing pill must still land, but the
	// guardian must NOT be woken (broadcast guardian id forced to 0).
	emitter.EmitChildEvent(chain.TenantID, chain.StudentID, chain.AccountID, parentmessaging.ChildEvent{
		EventType:      usersModels.ParentMessageEventRequestStatus,
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: chain.AccountID,
		Body:           "Anfrage bestätigt",
		RequestType:    usersModels.ParentMessageRequestMasterData,
		RequestStatus:  usersModels.ParentMessageRequestStatusDone,
		RefTable:       "users.student_data_change_requests",
		RefID:          i64(303),
	})

	_, msgs := threadPills(t, db, repos, chain)
	assert.Equal(t, 1, countEventType(msgs, usersModels.ParentMessageEventRequestStatus),
		"the closing pill is still written for the staff timeline")
	parentCalls := bc.CallsByMethod("parent")
	require.NotEmpty(t, parentCalls, "the tenant-wide staff wake still fires")
	assert.Equal(t, int64(0), parentCalls[len(parentCalls)-1].GuardianID,
		"a revoked guardian must not be woken (guardian broadcast id forced to 0)")
}
