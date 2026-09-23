package behavior_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #3477: removing a contact from the child does not remove its login link.
// Recreating the contact must not strand the parent's existing account.
func TestInviteToStudent_RecreatedContactRecoversOrphanedAccount(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, env.db, "reinvite-parent")
	actor := testpkg.CreateTestAccount(t, env.db, "reinvite-admin")
	student := testpkg.CreateTestStudent(t, env.db, "Reinvite", "Child", "1a")
	req := identityaccess.InviteToStudentRequest{
		StudentID: student.ID, Email: account.Email, FirstName: "Parent", LastName: "Contact", CreatedBy: actor.ID,
	}
	first, err := env.service.InviteToStudent(ctx, req)
	require.NoError(t, err)
	old, err := env.repos.GuardianProfile.FindByID(ctx, first.GuardianProfileID)
	require.NoError(t, err)
	previousEmail := "previous-" + account.Email
	old.Email = &previousEmail
	require.NoError(t, env.repos.GuardianProfile.Update(ctx, old))
	links, err := env.repos.StudentGuardian.FindByGuardianProfileID(ctx, old.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.NoError(t, env.repos.StudentGuardian.Delete(ctx, links[0].ID))

	replacement := testpkg.CreateTestGuardianProfile(t, env.db, "replacement-contact")
	replacement.Email = &account.Email
	require.NoError(t, env.repos.GuardianProfile.Update(ctx, replacement))
	testpkg.CreateTestStudentGuardianLink(t, env.db, student.ID, replacement.ID, "legal_guardian")
	before, err := env.repos.StudentGuardian.FindByGuardianProfileID(ctx, replacement.ID)
	require.NoError(t, err)

	result, err := env.service.InviteToStudent(ctx, req)
	require.NoError(t, err, "re-inviting must recover the childless profile's account association")
	assert.Equal(t, replacement.ID, result.GuardianProfileID)
	assert.Nil(t, result.InvitationID)
	old, err = env.repos.GuardianProfile.FindByID(ctx, old.ID)
	require.NoError(t, err)
	assert.Nil(t, old.AccountID)
	assert.False(t, old.HasAccount)
	assert.Equal(t, previousEmail, *old.Email, "the old contact is retained")
	replacement, err = env.repos.GuardianProfile.FindByID(ctx, replacement.ID)
	require.NoError(t, err)
	require.NotNil(t, replacement.AccountID)
	assert.Equal(t, account.ID, *replacement.AccountID)
	assert.True(t, replacement.HasAccount)
	after, err := env.repos.StudentGuardian.FindByGuardianProfileID(ctx, replacement.ID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, before[0].ID, after[0].ID)
	assert.Equal(t, before[0].Permissions, after[0].Permissions)
	_, err = env.service.InviteToStudent(ctx, req)
	require.NoError(t, err, "repeating the invitation is safe")
}

func TestInviteToStudent_RecreatedContactDoesNotDetachOtherChildren(t *testing.T) {
	t.Parallel()
	env := setupGuardianInvitationTest(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, env.db, "retained-parent")
	actor := testpkg.CreateTestAccount(t, env.db, "retained-admin")
	old := testpkg.CreateTestGuardianProfile(t, env.db, "retained-contact")
	require.NoError(t, env.repos.GuardianProfile.LinkAccount(ctx, old.ID, account.ID))
	child := testpkg.CreateTestStudent(t, env.db, "Retained", "Child", "1a")
	testpkg.CreateTestStudentGuardianLink(t, env.db, child.ID, old.ID, "legal_guardian")

	replacement := testpkg.CreateTestGuardianProfile(t, env.db, "conflicting-contact")
	replacement.Email = &account.Email
	require.NoError(t, env.repos.GuardianProfile.Update(ctx, replacement))
	_, err := env.service.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
		StudentID: child.ID, Email: account.Email, CreatedBy: actor.ID,
	})
	require.ErrorContains(t, err, "guardian account link conflicts with an existing profile")
	assert.Equal(t, services.GuardianFailureConflict, services.ClassifyGuardianInvitationFailure(err))
	assert.True(t, env.linkExists(t, child.ID, old.ID))
	assert.False(t, env.linkExists(t, child.ID, replacement.ID))
	old, err = env.repos.GuardianProfile.FindByID(ctx, old.ID)
	require.NoError(t, err)
	require.NotNil(t, old.AccountID)
	assert.Equal(t, account.ID, *old.AccountID)
}
