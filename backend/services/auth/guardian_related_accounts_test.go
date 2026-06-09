package auth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// linkExists reports whether a student↔guardian link row exists.
func (env *guardianTestEnv) linkExists(t *testing.T, studentID, guardianProfileID int64) bool {
	t.Helper()
	links, err := env.repos.StudentGuardian.FindByStudentID(testpkg.TenantContext(1), studentID)
	require.NoError(t, err)
	for _, l := range links {
		if l.GuardianProfileID == guardianProfileID {
			return true
		}
	}
	return false
}

func (env *guardianTestEnv) deleteStudentGuardianLinks(studentID int64) {
	_, _ = env.db.NewDelete().
		TableExpr("users.students_guardians").
		Where("student_id = ?", studentID).
		Exec(context.Background())
}

func TestInviteToStudent_NewEmail_CreatesProfileLinkAndInvite(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Invite", "Target", "1a")
	creatorID := env.inviterAccountID(t)
	email := fmt.Sprintf("new-guardian-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     email,
		FirstName: "Neue",
		LastName:  "Bezugsperson",
		CreatedBy: creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	defer func() {
		if result.InvitationID != nil {
			env.cleanupInvitation(t, *result.InvitationID, result.GuardianProfileID)
		}
	}()

	assert.Equal(t, authService.InviteOutcomeInvited, result.Outcome)
	assert.NotNil(t, result.InvitationID, "a token invitation should have been created")
	assert.Greater(t, result.GuardianProfileID, int64(0))
	assert.True(t, env.linkExists(t, student.ID, result.GuardianProfileID),
		"student↔guardian link must exist after invite")
}

func TestInviteToStudent_ExistingAccount_AutoLinks(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Sibling", "Parent", "2b")
	creatorID := env.inviterAccountID(t)
	defer env.deleteStudentGuardianLinks(student.ID)

	// A guardian who already has an account (sibling parent).
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "existing-acct")
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Existing", "Account")
	ctx := testpkg.TenantContext(1)
	require.NoError(t, env.repos.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     *profile.Email,
		CreatedBy: creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, authService.InviteOutcomeLinkedExistingAccount, result.Outcome)
	assert.Nil(t, result.InvitationID, "existing accounts are linked without a token invite")
	assert.Equal(t, profile.ID, result.GuardianProfileID)
	assert.True(t, env.linkExists(t, student.ID, profile.ID))
}

func TestRevokeAccess_ParentCannotRemovePrimary_StaffCan(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Primary", "Guard", "3c")
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "primary-guard")
	defer env.deleteStudentGuardianLinks(student.ID)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	// Link as the primary guardian.
	ctx := testpkg.TenantContext(1)
	link := &users.StudentGuardian{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "parent",
		IsPrimary:         true,
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	require.NoError(t, env.repos.StudentGuardian.Create(ctx, link))

	actorID := env.inviterAccountID(t)

	// Parent attempt → rejected, link survives.
	err := env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		ActorAccountID:    actorID,
		ByParent:          true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, authService.ErrCannotRemovePrimaryGuardian),
		"parents must not be able to remove the primary guardian")
	assert.True(t, env.linkExists(t, student.ID, profile.ID), "link must survive a rejected parent removal")

	// Staff attempt → succeeds, link gone.
	err = env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		ActorAccountID:    actorID,
		ByParent:          false,
	})
	require.NoError(t, err)
	assert.False(t, env.linkExists(t, student.ID, profile.ID), "staff removal must delete the link")
}

func TestInviteToStudent_RequireApproval_QueuesPending(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Approval", "Flow", "4d")
	creatorID := env.inviterAccountID(t)
	email := fmt.Sprintf("pending-guardian-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      email,
		CreatedBy:                  creatorID,
		RequestedByParentAccountID: &creatorID,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)
	defer env.cleanupInvitation(t, *result.InvitationID, result.GuardianProfileID)

	assert.Equal(t, authService.InviteOutcomePendingApproval, result.Outcome)
	// No link is created until approval.
	assert.False(t, env.linkExists(t, student.ID, result.GuardianProfileID),
		"pending approval must not link the child yet")

	// It shows up in the approval queue.
	pending, err := env.service.ListPendingApprovals(ctx)
	require.NoError(t, err)
	found := false
	for _, inv := range pending {
		if inv.ID == *result.InvitationID {
			found = true
			assert.Equal(t, authModels.GuardianInvitationApprovalPending, inv.ApprovalStatus)
		}
	}
	assert.True(t, found, "queued invitation must appear in ListPendingApprovals")

	// Approving it links the child.
	require.NoError(t, env.service.ApproveInvitation(ctx, *result.InvitationID, creatorID))
	assert.True(t, env.linkExists(t, student.ID, result.GuardianProfileID),
		"approval must link the child")
}

func TestListPendingApprovalsDetailed_ResolvesNames(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Mila", "Schmidt", "1a")
	requester := testpkg.CreateTestAccount(t, env.db, "requester-parent")
	email := fmt.Sprintf("invitee-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      email,
		FirstName:                  "Oma",
		LastName:                   "Schmidt",
		CreatedBy:                  requester.ID,
		RequestedByParentAccountID: &requester.ID,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)
	defer env.cleanupInvitation(t, *result.InvitationID, result.GuardianProfileID)

	views, err := env.service.ListPendingApprovalsDetailed(ctx)
	require.NoError(t, err)

	var view *authService.PendingApprovalView
	for _, v := range views {
		if v.InvitationID == *result.InvitationID {
			view = v
		}
	}
	require.NotNil(t, view, "queued invite must appear in the detailed approval queue")
	assert.Equal(t, email, view.GuardianEmail)
	assert.Equal(t, "Oma Schmidt", view.GuardianName)
	assert.Equal(t, student.ID, view.StudentID)
	assert.Equal(t, "Mila Schmidt", view.StudentName, "child name must be resolved via student→person")
	assert.Equal(t, requester.Email, view.RequestedByEmail, "requester email must be resolved")
}
