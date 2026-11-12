package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianInvitationIssuancePreservesUnrelatedApproval(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	for _, kind := range []string{"child invitation", "profile invitation"} {
		t.Run(kind, func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			creator := env.inviterAccountID(t)
			child := testpkg.CreateTestStudent(t, env.db, "Pending", "Access", "1a")
			email := fmt.Sprintf("pending-access-%d@example.test", child.ID)
			pending, err := env.service.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
				StudentID: child.ID, Email: email, CreatedBy: creator,
				RequestedByParentAccountID: &creator, RequireApproval: true,
			})
			require.NoError(t, err)
			require.NotNil(t, pending.InvitationID)
			before := testpkg.GuardianInvitationByID(t, env.db, *pending.InvitationID)
			if kind == "child invitation" {
				sibling := testpkg.CreateTestStudent(t, env.db, "Direct", "Access", "1b")
				_, err = env.service.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
					StudentID: sibling.ID, Email: email, CreatedBy: creator,
				})
			} else {
				_, err = env.service.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
					GuardianProfileID: pending.GuardianProfileID, CreatedBy: creator,
				})
			}
			require.NoError(t, err)
			after := testpkg.GuardianInvitationByID(t, env.db, *pending.InvitationID)
			assert.Equal(t, identityaccess.GuardianInvitationApprovalPending, after.ApprovalStatus)
			assert.Equal(t, before.ExpiresAt, after.ExpiresAt)
			assert.Equal(t, before.Email, after.Email)
			assert.False(t, env.linkExists(t, child.ID, pending.GuardianProfileID), "another invitation grants no access to this child")
			approvals, err := env.service.ListPendingApprovalsDetailed(ctx)
			require.NoError(t, err)
			require.Len(t, approvals, 1)
			assert.Equal(t, *pending.InvitationID, approvals[0].InvitationID)
		})
	}
}

func TestResendGuardianInvitation_LosesToConcurrentInvalidation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	outbox := api.NewCapturingOutbox()
	service, err := api.NewGuardianInvitationServiceForTests(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.GuardianInvitationTestConfig{Outbox: outbox})
	require.NoError(t, err)
	creator := testpkg.CreateTestAccount(t, db, "resend-invalidation")
	for _, status := range []string{identityaccess.GuardianInvitationApprovalNotRequired, identityaccess.GuardianInvitationApprovalApproved} {
		t.Run(status, func(t *testing.T) {
			profile := testpkg.CreateTestGuardianProfile(t, db, "resend-invalidation-"+status)
			invitation, err := service.CreateGuardianInvitation(testpkg.Ctx(t), identityaccess.GuardianInvitationRequest{
				GuardianProfileID: profile.ID, CreatedBy: creator.ID,
			})
			require.NoError(t, err)
			_, err = db.ExecContext(t.Context(), `UPDATE auth.guardian_invitations SET approval_status = ? WHERE id = ?`, status, invitation.ID)
			require.NoError(t, err)
			before := testpkg.GuardianInvitationByID(t, db, invitation.ID)
			mailCount := len(outbox.Requests())
			err = raceHeldTransition(t, db, func(context.Context) error {
				return service.ResendGuardianInvitation(testpkg.Ctx(t), invitation.ID, creator.ID)
			}, `UPDATE auth.guardian_invitations SET expires_at = NOW() - INTERVAL '1 minute', email = NULL WHERE id = ?`, invitation.ID)
			require.Error(t, err, "a resend cannot restore the expiry from its stale snapshot")
			after := testpkg.GuardianInvitationByID(t, db, invitation.ID)
			assert.Equal(t, before.Token, after.Token, "no new token replaces the invalidated link")
			assert.True(t, after.ExpiresAt.Before(time.Now()))
			assert.Empty(t, after.Email)
			assert.Len(t, outbox.Requests(), mailCount, "a refused resend queues no mail")
		})
	}
}
