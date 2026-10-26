package behavior_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// An invitation whose mail the outbox does not take is not kept (#4602): the
// caller gets the failure and nothing of the invitation commits. The enqueue
// fails with a statement on the invitation's transaction, the way a database
// error inside the outbox would.

func failingEnqueueStatement(ctx context.Context) error {
	raw, ok := testpkg.TransactionFromContext(ctx)
	if !ok {
		return errors.New("enqueue ran outside the invitation transaction")
	}
	_, err := raw.(bun.IDB).ExecContext(ctx, "SELECT 1/0")
	return err
}

func TestCreateGuardianInvitation_MailFailureRollsBackTheInvitation(t *testing.T) {
	t.Parallel()
	outbox := api.NewFailingCapturingOutbox(emailKindGuardianInvitation, failingEnqueueStatement)
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "mail-failure")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)

	err := testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := env.service.CreateGuardianInvitation(txCtx, identityaccess.GuardianInvitationRequest{
			GuardianProfileID: profile.ID, CreatedBy: creatorID,
		})
		return err
	})

	require.ErrorIs(t, err, identityaccess.ErrGuardianAccessMailNotQueued)
	assert.Empty(t, testpkg.GuardianInvitationsByProfile(t, env.db, profile.ID), "the invitation rolls back with its mail")
	assert.Empty(t, outbox.Requests(), "no welcome follows a failed invitation mail")
}

func TestResendGuardianInvitation_MailFailureKeepsThePreviousLink(t *testing.T) {
	t.Parallel()
	var failing atomic.Bool
	outbox := api.NewCapturingOutboxFailingWhen(func(ctx context.Context, kind string, _ map[string]any) error {
		if failing.Load() && kind == emailKindGuardianInvitation {
			return failingEnqueueStatement(ctx)
		}
		return nil
	})
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "resend-failure")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)
	invitation, err := env.service.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creatorID,
	})
	require.NoError(t, err)
	queued := len(outbox.Requests())

	failing.Store(true)
	err = env.service.ResendGuardianInvitation(ctx, invitation.ID, creatorID)

	require.ErrorIs(t, err, identityaccess.ErrGuardianAccessMailNotQueued, "the resend reports the failed mail")
	assert.Len(t, outbox.Requests(), queued)
	assert.Equal(t, sha256Hex(invitation.Token), testpkg.GuardianInvitationByID(t, env.db, invitation.ID).Token,
		"the new link rolls back; the previous one still works")
	_, err = env.service.ValidateGuardianInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
}

// In a bulk run only the guardian whose mail failed is rolled back and
// listed with the reason; the others are invited as usual.
func TestBulkInviteToStudents_MailFailureListsOnlyThatGuardian(t *testing.T) {
	t.Parallel()
	var failingEmail atomic.Value
	failingEmail.Store("")
	outbox := api.NewCapturingOutboxFailingWhen(func(ctx context.Context, kind string, payload map[string]any) error {
		if kind == emailKindGuardianInvitation && payload["recipient_email"] == failingEmail.Load() {
			return failingEnqueueStatement(ctx)
		}
		return nil
	})
	env := setupGuardianInvitationTest(t, func(_ *bun.DB, cfg *api.GuardianInvitationTestConfig) { cfg.Outbox = outbox })

	first := testpkg.CreateTestStudent(t, env.db, "Bulk", "Erste", "2a")
	second := testpkg.CreateTestStudent(t, env.db, "Bulk", "Zweite", "2b")
	reached := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Erreicht", "Muster", "bulk-reached")
	unreached := testpkg.CreateTestGuardianProfileNamed(t, env.db, "Nicht", "Erreicht", "bulk-unreached")
	testpkg.CreateTestStudentGuardianLink(t, env.db, first.ID, reached.ID, guardianRoleLegalGuardian)
	testpkg.CreateTestStudentGuardianLink(t, env.db, second.ID, unreached.ID, guardianRoleLegalGuardian)
	require.NotNil(t, unreached.Email)
	failingEmail.Store(*unreached.Email)

	result, err := env.service.BulkInviteToStudents(testpkg.Ctx(t), identityaccess.BulkInviteRequest{
		StudentIDs: []int64{first.ID, second.ID}, CreatedBy: env.inviterAccountID(t),
	})

	require.NoError(t, err, "one failed mail does not fail the run")
	assert.Equal(t, 1, result.Invited)
	require.Len(t, result.Problems, 1)
	assert.Equal(t, unreached.ID, result.Problems[0].GuardianProfileID)
	assert.Equal(t, identityaccess.BulkInviteProblemEmailNotQueued, result.Problems[0].Reason)
	assert.NotEmpty(t, result.Problems[0].StudentNames, "the problem names the child")
	assert.Empty(t, testpkg.GuardianInvitationsByProfile(t, env.db, unreached.ID), "the failed guardian is rolled back")
	assert.Len(t, testpkg.GuardianInvitationsByProfile(t, env.db, reached.ID), 1, "the other guardian is invited")
}
