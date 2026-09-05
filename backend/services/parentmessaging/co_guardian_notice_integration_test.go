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

// #2267 story 47. A decision on one guardian's request changes the child's
// care for the whole family, but only the submitter used to hear about it —
// the other parent found out when the child was not picked up. Every other
// portal guardian now gets a NEUTRAL line, and neutral is the load-bearing
// word: no reason, no author, no deep link into a request they may not read.
func TestEmitChildEventToOtherGuardians_NeutralForEveryoneButTheSubmitter(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	submitter := testpkg.CreateTestParentGuardianChain(t, db)
	reviewer := testpkg.CreateTestAccount(t, db, "co-guardian-notice-reviewer")
	other := testpkg.CreateTestCoGuardianForStudent(t, db, submitter.StudentID, "Klaus", "Zweitelternteil")

	settings := &toggleSettings{enabled: true}
	emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
		settings, testpkg.NewRecordingBroadcaster(), slog.Default())

	full := parentmessaging.ChildEvent{
		EventType:      "request_status",
		ActorKind:      usersModels.ParentMessageSenderStaff,
		ActorAccountID: reviewer.ID,
		Body:           "Betreuungsstand geändert: Krankmeldung 01.09.2026",
		RequestType:    "absence",
		RequestStatus:  usersModels.ParentMessageRequestStatusDone,
		DecisionReason: "Attest liegt vor",
		RefTable:       "active.excused_absence_requests",
		RefID:          i64(99),
	}
	recipients := otherGuardiansOf(t, db, emitter, submitter)
	require.Equal(t, []int64{other.AccountID}, recipients)
	emitter.EmitChildEventToGuardians(submitter.TenantID, submitter.StudentID, recipients, full)

	// The other guardian gets exactly one line, and it carries nothing that
	// belongs to the submitting parent.
	_, otherPills := threadPills(t, db, repos, other)
	require.Len(t, otherPills, 1)
	assert.Equal(t, "Betreuungsstand geändert: Krankmeldung 01.09.2026", otherPills[0].Body)
	assert.Empty(t, otherPills[0].DecisionReason, "the staff reason belongs to the submitting guardian")
	assert.Empty(t, otherPills[0].RequestStatus, "a co-guardian filed no request to have a status")
	assert.Empty(t, otherPills[0].RefTable, "no deep link into a request this guardian may not read")

	// The submitter is skipped here: their full pill comes from the decision
	// itself, so a fan-out line would duplicate it.
	submitterThread, _ := threadPills(t, db, repos, submitter)
	assert.Nil(t, submitterThread, "the submitter must not receive the neutral fan-out")
}

// A guardian without portal access is not in the thread guardian list at all,
// so the fan-out reaches nobody and creates no thread.
func TestEmitChildEventToOtherGuardians_LoneGuardianGetsNothing(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	submitter := testpkg.CreateTestParentGuardianChain(t, db)

	settings := &toggleSettings{enabled: true}
	emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
		settings, testpkg.NewRecordingBroadcaster(), slog.Default())

	recipients := otherGuardiansOf(t, db, emitter, submitter)
	assert.Empty(t, recipients, "a child with one guardian has nobody to notify")
	emitter.EmitChildEventToGuardians(submitter.TenantID, submitter.StudentID, recipients,
		parentmessaging.ChildEvent{
			EventType: "request_status", ActorKind: usersModels.ParentMessageSenderStaff,
			Body: "Betreuungsstand geändert: Krankmeldung 01.09.2026",
		})

	thread, _ := threadPills(t, db, repos, submitter)
	assert.Nil(t, thread, "no other guardian means no pill and no thread")
}

// otherGuardiansOf resolves the fan-out recipients the way production does:
// inside a tenant transaction, on the caller's context.
func otherGuardiansOf(
	t *testing.T, db *bun.DB, emitter *parentmessaging.Emitter, submitter testpkg.ParentChain,
) []int64 {
	t.Helper()
	var accountIDs []int64
	err := tenant.WithTenantTx(
		testpkg.WithTenantRuntime(t, context.Background(), db), db, submitter.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			var resolveErr error
			accountIDs, resolveErr = emitter.OtherPortalGuardianAccountIDs(
				txCtx, submitter.StudentID, submitter.AccountID,
			)
			return resolveErr
		})
	require.NoError(t, err)
	return accountIDs
}

// #2267 story 47: a guardian the parent explicitly shared the request with
// already sees the request, so withholding the reason from them would make the
// pill useless. They get the SAME pill the submitter gets; everyone else gets
// the neutral line.
func TestResolveDecisionAudience_SplitsSharedFromNeutral(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	submitter := testpkg.CreateTestParentGuardianChain(t, db)
	shared := testpkg.CreateTestCoGuardianForStudent(t, db, submitter.StudentID, "Klaus", "Mitleser")
	uninvolved := testpkg.CreateTestCoGuardianForStudent(t, db, submitter.StudentID, "Rita", "Ohnezugang")

	emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default())

	var audience parentmessaging.DecisionAudience
	require.NoError(t, tenant.WithTenantTx(
		testpkg.WithTenantRuntime(t, context.Background(), db), db, submitter.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			var err error
			audience, err = emitter.ResolveDecisionAudience(
				txCtx, submitter.StudentID, submitter.AccountID, []int64{shared.AccountID},
			)
			return err
		}))

	assert.Equal(t, []int64{shared.AccountID}, audience.Full)
	assert.Equal(t, []int64{uninvolved.AccountID}, audience.Neutral)
	assert.NotContains(t, audience.Full, submitter.AccountID, "the submitter is never in the fan-out")
	assert.NotContains(t, audience.Neutral, submitter.AccountID)
}

// An unwired or failing sharing resolver reports no explicit recipients, which
// puts everyone in Neutral. That is the safe direction: seeing less than you
// were entitled to is a nuisance, seeing more is a leak. Familienschutz lands
// here by the same route, because it voids every share while it is on.
func TestResolveDecisionAudience_NoSharesMeansEverybodyNeutral(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	submitter := testpkg.CreateTestParentGuardianChain(t, db)
	other := testpkg.CreateTestCoGuardianForStudent(t, db, submitter.StudentID, "Klaus", "Zweitelternteil")

	emitter := newMockEmitter(t, db, repos.ParentMessageThread, repos.ParentMessage,
		&toggleSettings{enabled: true}, testpkg.NewRecordingBroadcaster(), slog.Default())

	var audience parentmessaging.DecisionAudience
	require.NoError(t, tenant.WithTenantTx(
		testpkg.WithTenantRuntime(t, context.Background(), db), db, submitter.TenantID,
		func(txCtx context.Context, _ bun.Tx) error {
			var err error
			audience, err = emitter.ResolveDecisionAudience(
				txCtx, submitter.StudentID, submitter.AccountID, nil,
			)
			return err
		}))

	assert.Empty(t, audience.Full)
	assert.Equal(t, []int64{other.AccountID}, audience.Neutral)
}
