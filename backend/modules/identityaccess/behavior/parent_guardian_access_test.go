package behavior_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
)

// recordingRelativeAccess captures what the root forwards to the owner.
type recordingRelativeAccess struct {
	identityaccess.GuardianRelativeAccess
	invite *identityaccess.InviteToStudentRequest
	revoke *identityaccess.RevokeAccessRequest
}

func (r *recordingRelativeAccess) InviteToStudent(_ context.Context, request identityaccess.InviteToStudentRequest) (*identityaccess.InviteToStudentResult, error) {
	r.invite = &request
	return &identityaccess.InviteToStudentResult{Outcome: identityaccess.InviteOutcomeInvited, GuardianProfileID: 7}, nil
}

func (r *recordingRelativeAccess) RevokeAccess(_ context.Context, request identityaccess.RevokeAccessRequest) error {
	r.revoke = &request
	return nil
}

// The parents portal never holds staff authority, so every removal it makes
// must reach the owner as a parent removal: that flag is what keeps the
// primary-guardian and payer protections in force (#2608, #3332).
func TestParentGuardianAccessRevokesAsParent(t *testing.T) {
	t.Parallel()

	owner := &recordingRelativeAccess{}
	access := services.NewParentGuardianAccess(owner)
	require.NotNil(t, access)

	require.NoError(t, access.RevokeAccess(context.Background(), services.GuardianAccessRevocation{
		StudentID: 11, GuardianProfileID: 22, ActorAccountID: 33,
	}))

	require.NotNil(t, owner.revoke)
	assert.True(t, owner.revoke.ByParent, "the parents portal removes as a parent, never as staff")
	assert.False(t, owner.revoke.MayClearPayer, "the parents portal never holds guardians:financial")
	assert.Equal(t, int64(11), owner.revoke.StudentID)
	assert.Equal(t, int64(22), owner.revoke.GuardianProfileID)
	assert.Equal(t, int64(33), owner.revoke.ActorAccountID)
}

// A parent-initiated invite reaches the owner marked as such, which is what
// puts it in the staff approval queue instead of linking immediately.
func TestParentGuardianAccessMarksTheRequestingParent(t *testing.T) {
	t.Parallel()

	owner := &recordingRelativeAccess{}
	access := services.NewParentGuardianAccess(owner)
	require.NotNil(t, access)

	outcome, err := access.InviteToStudent(context.Background(), services.GuardianInviteRequest{
		StudentID: 11, Email: "oma@example.test", CreatedBy: 33, RequestedByAccountID: 33,
		RequireApproval: true, ConfirmRoleUpgrade: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "invited", outcome.Outcome)

	require.NotNil(t, owner.invite)
	require.NotNil(t, owner.invite.RequestedByParentAccountID, "a parent-initiated invite names its requester")
	assert.Equal(t, int64(33), *owner.invite.RequestedByParentAccountID)
	assert.True(t, owner.invite.RequireApproval)
	assert.True(t, owner.invite.ConfirmRoleUpgrade)
}

// A root composed without the capability leaves the port nil, which the
// portal reports as unavailable instead of panicking.
func TestParentGuardianAccessWithoutCapability(t *testing.T) {
	t.Parallel()
	assert.Nil(t, services.NewParentGuardianAccess(nil))
}
