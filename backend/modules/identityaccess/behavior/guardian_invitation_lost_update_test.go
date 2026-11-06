package behavior_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Guardian invitation transitions read the row without a lock. A transition
// that commits between that read and the write must not be overwritten.

// A staff approval racing a rejection: the rejection committed first, so the
// approval must fail and leave no child link behind.
func TestApproveInvitation_LosesToConcurrentRejection(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	student := testpkg.CreateTestStudent(t, env.db, "Race", "Approval", "3c")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)
	result, err := env.service.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
		StudentID: student.ID, Email: fmt.Sprintf("race-approval-%d@example.test", time.Now().UnixNano()),
		FirstName: "Race", LastName: "Guardian", CreatedBy: creatorID,
		RequestedByParentAccountID: &creatorID, RequireApproval: true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)
	invitationID := *result.InvitationID

	err = raceHeldTransition(t, env.db, func(context.Context) error {
		return env.service.ApproveInvitation(ctx, invitationID, creatorID)
	}, `UPDATE auth.guardian_invitations SET approval_status = 'rejected', approved_by = ?, approved_at = NOW() WHERE id = ?`,
		creatorID, invitationID)

	require.Error(t, err, "the approval must not overwrite the committed rejection")
	var status string
	require.NoError(t, env.db.NewRaw(`SELECT approval_status FROM auth.guardian_invitations WHERE id = ?`, invitationID).
		Scan(context.Background(), &status))
	assert.Equal(t, "rejected", status)
	assert.False(t, env.linkExists(t, student.ID, result.GuardianProfileID), "a rejected request grants no child access")
}

// A resend racing an acceptance: the acceptance committed first, so the
// resend must not reopen the spent invitation with a fresh link.
func TestResendGuardianInvitation_LosesToConcurrentAcceptance(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "race-resend")
	creatorID := env.inviterAccountID(t)
	ctx := testpkg.Ctx(t)
	invitation, err := env.service.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
		GuardianProfileID: profile.ID, CreatedBy: creatorID,
	})
	require.NoError(t, err)
	spentHash := testpkg.GuardianInvitationByID(t, env.db, invitation.ID).Token

	err = raceHeldTransition(t, env.db, func(context.Context) error {
		return env.service.ResendGuardianInvitation(ctx, invitation.ID, creatorID)
	}, `UPDATE auth.guardian_invitations SET accepted_at = NOW() WHERE id = ?`, invitation.ID)

	require.Error(t, err, "the resend must not overwrite the committed acceptance")
	stored := testpkg.GuardianInvitationByID(t, env.db, invitation.ID)
	assert.NotNil(t, stored.AcceptedAt, "the invitation stays spent")
	assert.Equal(t, spentHash, stored.Token, "no new link is issued for a spent invitation")
}
