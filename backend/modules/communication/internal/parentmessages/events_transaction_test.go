package messaging_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"
	messaging "github.com/moto-nrw/project-phoenix/modules/communication/internal/parentmessages"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestTimelinePillsShareTriggeringTransactionAndOnlySSEWaitsForCommit(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"request_created", "request_status", "sick_note", "care_exception", "care_exception_correction", "co_guardians"} {
		t.Run(kind, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			repos := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			broadcaster := testpkg.NewRecordingBroadcaster()
			emitter := newMockEmitter(t, db, messaging.NewThreadRepository(repos.ParentMessageThread), messaging.NewMessageRepository(parentStore.NewParentMessageRepository(db)), &toggleSettings{enabled: true}, broadcaster, slog.Default())
			event := eventContract.ChildEvent{EventType: kind, ActorKind: messaging.ParentMessageSenderGuardian, ActorAccountID: chain.AccountID, Body: "Transactional timeline fixture"}
			want := 1
			audience := eventContract.DecisionAudience{}
			if kind == "request_created" || kind == "request_status" {
				event.RequestType = messaging.ParentMessageRequestMasterData
				event.RequestStatus = messaging.ParentMessageRequestStatusOpen
			}
			if kind == "request_status" {
				event.RequestStatus = messaging.ParentMessageRequestStatusDone
			}
			if kind == "co_guardians" {
				other := testpkg.CreateTestCoGuardianForStudent(t, db, chain.StudentID, "Atomic", "Guardian")
				audience = eventContract.DecisionAudience{Full: []int64{chain.AccountID}, Neutral: []int64{other.AccountID}}
				event.EventType = "care_exception"
				want = 2
			}
			ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
			count := func(source bun.IDB, queryCtx context.Context, expected int) {
				t.Helper()
				n, err := source.NewSelect().TableExpr("users.parent_messages").Where("tenant_id = ? AND student_id = ?", chain.TenantID, chain.StudentID).Count(queryCtx)
				require.NoError(t, err)
				assert.EqualValues(t, expected, n)
			}
			rollback := errors.New("rollback triggering care change")
			trigger := func(commit bool) error {
				return testpkg.WithinTenantTransaction(ctx, db, chain.TenantID, func(txCtx context.Context, tx bun.Tx) error {
					var err error
					if kind == "co_guardians" {
						err = emitter.EmitDecisionAudience(txCtx, chain.TenantID, chain.StudentID, audience, event, event)
					} else {
						err = emitter.EmitChildEvent(txCtx, chain.TenantID, chain.StudentID, chain.AccountID, event)
					}
					if err != nil {
						return err
					}
					count(tx, txCtx, want)
					assert.Empty(t, broadcaster.Calls(), "only realtime fan-out waits for commit")
					if !commit {
						return rollback
					}
					return nil
				})
			}
			require.ErrorIs(t, trigger(false), rollback)
			count(db, ctx, 0)
			assert.Empty(t, broadcaster.Calls())
			require.NoError(t, trigger(true))
			count(db, ctx, want)
			assert.NotEmpty(t, broadcaster.Calls())
		})
	}
}
