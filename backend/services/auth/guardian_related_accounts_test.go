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

func TestInviteToStudent_ExistingAccountWithoutTenantProfile_AutoLinks(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Cross", "Tenant", "2c")
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Cross", "Account")
	creatorID := env.inviterAccountID(t)
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     account.Email,
		CreatedBy: creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", result.GuardianProfileID).Exec(context.Background())
	}()

	assert.Equal(t, authService.InviteOutcomeLinkedExistingAccount, result.Outcome)
	assert.Nil(t, result.InvitationID, "existing accounts are linked without a password invite")
	assert.True(t, env.linkExists(t, student.ID, result.GuardianProfileID))

	profile, err := env.repos.GuardianProfile.FindByID(ctx, result.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, profile.AccountID)
	assert.Equal(t, account.ID, *profile.AccountID)
	assert.True(t, profile.HasAccount)
}

func TestInviteToStudent_RequireApprovalExistingAccountWithoutTenantProfile_DefersAttachment(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Pending", "Existing", "2d")
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Pending", "Account")
	creatorID := env.inviterAccountID(t)
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      account.Email,
		CreatedBy:                  creatorID,
		RequestedByParentAccountID: &creatorID,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)
	defer env.cleanupInvitation(t, *result.InvitationID, result.GuardianProfileID)

	assert.Equal(t, authService.InviteOutcomePendingApproval, result.Outcome)
	assert.False(t, env.linkExists(t, student.ID, result.GuardianProfileID), "staff approval must not create the child link")

	profile, err := env.repos.GuardianProfile.FindByID(ctx, result.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.False(t, profile.HasAccount, "existing account must not be attached before staff approval")
	assert.Nil(t, profile.AccountID)

	require.NoError(t, env.service.ApproveInvitation(ctx, *result.InvitationID, creatorID))
	assert.True(t, env.linkExists(t, student.ID, result.GuardianProfileID), "approval must create the child link")
	profile, err = env.repos.GuardianProfile.FindByID(ctx, result.GuardianProfileID)
	require.NoError(t, err)
	require.NotNil(t, profile.AccountID)
	assert.Equal(t, account.ID, *profile.AccountID)
	assert.True(t, profile.HasAccount)
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

func TestRevokeAccess_ParentCannotRemoveStaffManagedNoAccountContact(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Staff", "Contact", "3d")
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "staff-managed-no-account")
	actorID := env.inviterAccountID(t)
	defer env.deleteStudentGuardianLinks(student.ID)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	ctx := testpkg.TenantContext(1)
	link := &users.StudentGuardian{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	require.NoError(t, env.repos.StudentGuardian.Create(ctx, link))

	err := env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		ActorAccountID:    actorID,
		ByParent:          true,
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, authService.ErrCannotRemoveStaffManagedGuardian))
	assert.True(t, env.linkExists(t, student.ID, profile.ID), "staff-maintained contact link must survive")
}

func TestRevokeAccess_ParentCancelsInviteForStaffManagedContactWithoutDeletingLink(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Staff", "Pending", "3f")
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "staff-managed-pending")
	actorID := env.inviterAccountID(t)
	defer env.deleteStudentGuardianLinks(student.ID)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	ctx := testpkg.TenantContext(1)
	link := &users.StudentGuardian{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "guardian",
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	require.NoError(t, env.repos.StudentGuardian.Create(ctx, link))
	studentID := student.ID
	invitation := &authModels.GuardianInvitation{
		Token:             fmt.Sprintf("staff-contact-open-invite-%d", time.Now().UnixNano()),
		GuardianProfileID: profile.ID,
		CreatedBy:         actorID,
		ExpiresAt:         time.Now().Add(time.Hour),
		StudentID:         &studentID,
		ApprovalStatus:    authModels.GuardianInvitationApprovalNotRequired,
	}
	invitation.SetTenantID(1)
	require.NoError(t, env.repos.GuardianInvitation.Create(ctx, invitation))

	require.NoError(t, env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		ActorAccountID:    actorID,
		ByParent:          true,
	}))
	assert.True(t, env.linkExists(t, student.ID, profile.ID), "cancelling portal invite must not delete staff contact data")

	updated, err := env.repos.GuardianInvitation.FindByID(context.Background(), invitation.ID)
	require.NoError(t, err)
	assert.False(t, updated.ExpiresAt.After(time.Now()), "pending portal token must be invalidated")
}

func TestRevokeAccess_ParentPendingInviteRemovalExpiresToken(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Pending", "Remove", "3e")
	creatorID := env.inviterAccountID(t)
	email := fmt.Sprintf("pending-remove-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      email,
		CreatedBy:                  creatorID,
		RequestedByParentAccountID: &creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)
	defer env.cleanupInvitation(t, *result.InvitationID, result.GuardianProfileID)
	assert.True(t, env.linkExists(t, student.ID, result.GuardianProfileID))

	require.NoError(t, env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{
		StudentID:         student.ID,
		GuardianProfileID: result.GuardianProfileID,
		ActorAccountID:    creatorID,
		ByParent:          true,
	}))
	assert.False(t, env.linkExists(t, student.ID, result.GuardianProfileID), "pending access link must be removed")

	inv, err := env.repos.GuardianInvitation.FindByID(context.Background(), *result.InvitationID)
	require.NoError(t, err)
	assert.False(t, inv.ExpiresAt.After(time.Now()), "removing pending access must invalidate the token")
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
	pending, err := env.service.ListPendingApprovalsDetailed(ctx)
	require.NoError(t, err)
	found := false
	for _, view := range pending {
		if view.InvitationID == *result.InvitationID {
			found = true
		}
	}
	assert.True(t, found, "queued invitation must appear in the approval queue")

	// Approving it links the child.
	require.NoError(t, env.service.ApproveInvitation(ctx, *result.InvitationID, creatorID))
	assert.True(t, env.linkExists(t, student.ID, result.GuardianProfileID),
		"approval must link the child")
}

func TestInviteToStudent_DirectInvitePromotesPendingApproval(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Promote", "Pending", "4e")
	creatorID := env.inviterAccountID(t)
	email := fmt.Sprintf("promote-pending-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	pending, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      email,
		CreatedBy:                  creatorID,
		RequestedByParentAccountID: &creatorID,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, pending.InvitationID)
	defer env.cleanupInvitation(t, *pending.InvitationID, pending.GuardianProfileID)

	direct, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     email,
		CreatedBy: creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, direct.InvitationID)
	assert.Equal(t, *pending.InvitationID, *direct.InvitationID, "staff/direct invite should reuse the pending token")
	assert.True(t, env.linkExists(t, student.ID, pending.GuardianProfileID), "staff/direct invite should create the child link")

	invitation, err := env.repos.GuardianInvitation.FindByID(context.Background(), *pending.InvitationID)
	require.NoError(t, err)
	assert.Equal(t, authModels.GuardianInvitationApprovalNotRequired, invitation.ApprovalStatus)
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

func TestRejectInvitation_MarksRejectedAndCleansOrphan(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Reject", "Flow", "5e")
	creatorID := env.inviterAccountID(t)
	email := fmt.Sprintf("rejected-guardian-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	ctx := testpkg.TenantContext(1)
	// Parent-initiated, requires approval → creates a fresh orphan profile +
	// a pending invitation, but no link.
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      email,
		FirstName:                  "Orphan",
		LastName:                   "Guardian",
		CreatedBy:                  creatorID,
		RequestedByParentAccountID: &creatorID,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)

	require.NoError(t, env.service.RejectInvitation(ctx, *result.InvitationID, creatorID))

	// No access granted: no link to the child.
	assert.False(t, env.linkExists(t, student.ID, result.GuardianProfileID))

	// The profile created only to back this request (no account, no links) is
	// cleaned up. Deleting it cascades to the invitation row (FK ON DELETE
	// CASCADE), so the whole speculative record disappears on reject.
	profile, _ := env.repos.GuardianProfile.FindByID(ctx, result.GuardianProfileID)
	assert.Nil(t, profile, "orphan profile must be removed on reject")
	inv, _ := env.repos.GuardianInvitation.FindByID(context.Background(), *result.InvitationID)
	assert.Nil(t, inv, "the rejected orphan invitation is cascade-deleted with its profile")
}

func TestApproveInvitation_ExistingAccount_LinksWithoutEmail(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	student := testpkg.CreateTestStudent(t, env.db, "Approve", "Existing", "6f")
	// A guardian who already has an account.
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "approve-existing")
	_, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Has", "Account")
	creatorID := env.inviterAccountID(t)
	defer env.deleteStudentGuardianLinks(student.ID)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	ctx := testpkg.TenantContext(1)
	require.NoError(t, env.repos.GuardianProfile.LinkAccount(ctx, profile.ID, account.ID))

	// Parent-initiated, requires approval → pending, no link yet.
	result, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      *profile.Email,
		CreatedBy:                  creatorID,
		RequestedByParentAccountID: &creatorID,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.InvitationID)
	defer env.cleanupInvitation(t, *result.InvitationID, profile.ID)
	assert.Equal(t, profile.ID, result.GuardianProfileID)
	assert.False(t, env.linkExists(t, student.ID, profile.ID), "no link before approval")

	// Approve → links the child to the existing account, no token email needed.
	require.NoError(t, env.service.ApproveInvitation(ctx, *result.InvitationID, creatorID))
	assert.True(t, env.linkExists(t, student.ID, profile.ID), "approval must link the child")

	inv, err := env.repos.GuardianInvitation.FindByID(context.Background(), *result.InvitationID)
	require.NoError(t, err)
	assert.Equal(t, authModels.GuardianInvitationApprovalApproved, inv.ApprovalStatus)
	assert.NotNil(t, inv.AcceptedAt, "existing-account approval closes the invitation")
}

func TestInviteToStudent_ValidationErrors(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	creator := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Val", "Idation", "1a")

	_, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{StudentID: 0, Email: "a@b.de", CreatedBy: creator})
	require.Error(t, err, "missing student ID")

	_, err = env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{StudentID: student.ID, Email: "a@b.de", CreatedBy: 0})
	require.Error(t, err, "missing created_by")

	_, err = env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{StudentID: student.ID, Email: "   ", CreatedBy: creator})
	require.Error(t, err, "blank email")
}

func TestApproveRejectInvitation_NotFoundAndNotPending(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	approver := env.inviterAccountID(t)

	require.Error(t, env.service.ApproveInvitation(ctx, 99999999, approver), "approve unknown id")
	require.Error(t, env.service.RejectInvitation(ctx, 99999999, approver), "reject unknown id")

	// A direct invite is approval_status=not_required → cannot be approved/rejected.
	student := testpkg.CreateTestStudent(t, env.db, "NotP", "Ending", "2b")
	defer env.deleteStudentGuardianLinks(student.ID)
	email := fmt.Sprintf("notpending-%d@example.test", time.Now().UnixNano())
	res, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{StudentID: student.ID, Email: email, CreatedBy: approver})
	require.NoError(t, err)
	require.NotNil(t, res.InvitationID)
	defer env.cleanupInvitation(t, *res.InvitationID, res.GuardianProfileID)

	require.Error(t, env.service.ApproveInvitation(ctx, *res.InvitationID, approver), "not awaiting approval")
	require.Error(t, env.service.RejectInvitation(ctx, *res.InvitationID, approver), "not awaiting approval")
}

func TestInviteToStudent_ReusesOpenInvitationForSameChildAndProfile(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	creator := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Repeat", "Invite", "4a")
	email := fmt.Sprintf("repeat-invite-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	first, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     email,
		CreatedBy: creator,
	})
	require.NoError(t, err)
	require.NotNil(t, first.InvitationID)
	defer env.cleanupInvitation(t, *first.InvitationID, first.GuardianProfileID)

	second, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     email,
		CreatedBy: creator,
	})
	require.NoError(t, err)
	require.NotNil(t, second.InvitationID)

	assert.Equal(t, first.GuardianProfileID, second.GuardianProfileID)
	assert.Equal(t, *first.InvitationID, *second.InvitationID, "repeat invite should reuse the open invitation")

	invitations, err := env.repos.GuardianInvitation.FindByGuardianProfileID(ctx, first.GuardianProfileID)
	require.NoError(t, err)
	openForStudent := 0
	now := time.Now()
	for _, inv := range invitations {
		if inv.StudentID != nil && *inv.StudentID == student.ID && authService.GuardianInvitationValid(inv, now) {
			openForStudent++
		}
	}
	assert.Equal(t, 1, openForStudent)
}

func TestInviteToStudent_ReplacesRevokedInvitation(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	creator := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Replacement", "Invite", "4b")
	email := fmt.Sprintf("replacement-invite-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(student.ID)

	first, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     email,
		CreatedBy: creator,
	})
	require.NoError(t, err)
	require.NotNil(t, first.InvitationID)
	defer func() {
		_, _ = env.db.NewDelete().
			TableExpr("auth.guardian_invitations").
			Where("guardian_profile_id = ?", first.GuardianProfileID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().
			TableExpr("users.guardian_profiles").
			Where("id = ?", first.GuardianProfileID).
			Exec(context.Background())
	}()

	revokedIDs, err := env.repos.GuardianInvitation.RevokeOpenByGuardianProfileID(ctx, first.GuardianProfileID, "guardian email changed")
	require.NoError(t, err)
	require.Contains(t, revokedIDs, *first.InvitationID)

	second, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID: student.ID,
		Email:     email,
		CreatedBy: creator,
	})
	require.NoError(t, err)
	require.NotNil(t, second.InvitationID)
	assert.NotEqual(t, *first.InvitationID, *second.InvitationID, "a revoked token must be replaced")

	replacement, err := env.repos.GuardianInvitation.FindByID(ctx, *second.InvitationID)
	require.NoError(t, err)
	assert.Nil(t, replacement.RevokedAt)
}

func TestRevokeAccess_ValidationAndNotLinked(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	actor := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Rev", "Oke", "3c")
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "not-linked")
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	require.Error(t, env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{StudentID: 0, GuardianProfileID: profile.ID, ActorAccountID: actor}), "missing ids")
	require.Error(t, env.service.RevokeAccess(ctx, authService.RevokeAccessRequest{StudentID: student.ID, GuardianProfileID: profile.ID, ActorAccountID: actor}), "not linked")
}

func TestRejectInvitation_PreservesProfileWithOtherLinks(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	approver := env.inviterAccountID(t)
	studentA := testpkg.CreateTestStudent(t, env.db, "Sib", "Aaa", "1a")
	studentB := testpkg.CreateTestStudent(t, env.db, "Sib", "Bbb", "1b")
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "multi-link")
	defer env.deleteStudentGuardianLinks(studentA.ID)
	defer env.deleteStudentGuardianLinks(studentB.ID)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	// Existing link to student A.
	link := &users.StudentGuardian{StudentID: studentA.ID, GuardianProfileID: profile.ID, RelationshipType: "parent", EmergencyPriority: 1}
	link.SetTenantID(1)
	require.NoError(t, env.repos.StudentGuardian.Create(ctx, link))

	// Pending request for the SAME guardian, targeting student B.
	res, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  studentB.ID,
		Email:                      *profile.Email,
		CreatedBy:                  approver,
		RequestedByParentAccountID: &approver,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, res.InvitationID)
	require.Equal(t, profile.ID, res.GuardianProfileID)
	defer env.cleanupInvitation(t, *res.InvitationID, profile.ID)

	require.NoError(t, env.service.RejectInvitation(ctx, *res.InvitationID, approver))
	// Profile is NOT cleaned up: it still has its link to student A.
	p, _ := env.repos.GuardianProfile.FindByID(ctx, profile.ID)
	assert.NotNil(t, p, "a guardian with other child links must survive a reject")
}

func TestRejectInvitation_PreservesPreexistingProfileWithoutLinks(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	approver := env.inviterAccountID(t)
	student := testpkg.CreateTestStudent(t, env.db, "Existing", "Contact", "1a")
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "preexisting-contact")
	defer env.deleteStudentGuardianLinks(student.ID)
	defer func() {
		_, _ = env.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
	}()

	res, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  student.ID,
		Email:                      *profile.Email,
		CreatedBy:                  approver,
		RequestedByParentAccountID: &approver,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, res.InvitationID)
	require.Equal(t, profile.ID, res.GuardianProfileID)
	defer env.cleanupInvitation(t, *res.InvitationID, profile.ID)

	require.NoError(t, env.service.RejectInvitation(ctx, *res.InvitationID, approver))
	p, err := env.repos.GuardianProfile.FindByID(ctx, profile.ID)
	require.NoError(t, err)
	assert.NotNil(t, p, "pre-existing contact profiles must survive reject cleanup")
}

func TestRejectInvitation_PreservesSharedPendingProfile(t *testing.T) {
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()
	ctx := testpkg.TenantContext(1)
	approver := env.inviterAccountID(t)
	studentA := testpkg.CreateTestStudent(t, env.db, "Shared", "Aaa", "1a")
	studentB := testpkg.CreateTestStudent(t, env.db, "Shared", "Bbb", "1b")
	email := fmt.Sprintf("shared-pending-%d@example.test", time.Now().UnixNano())
	defer env.deleteStudentGuardianLinks(studentA.ID)
	defer env.deleteStudentGuardianLinks(studentB.ID)

	first, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  studentA.ID,
		Email:                      email,
		FirstName:                  "Shared",
		LastName:                   "Pending",
		CreatedBy:                  approver,
		RequestedByParentAccountID: &approver,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, first.InvitationID)

	second, err := env.service.InviteToStudent(ctx, authService.InviteToStudentRequest{
		StudentID:                  studentB.ID,
		Email:                      email,
		CreatedBy:                  approver,
		RequestedByParentAccountID: &approver,
		RequireApproval:            true,
	})
	require.NoError(t, err)
	require.NotNil(t, second.InvitationID)
	require.Equal(t, first.GuardianProfileID, second.GuardianProfileID)
	defer env.cleanupInvitation(t, *first.InvitationID, first.GuardianProfileID)
	defer env.cleanupInvitation(t, *second.InvitationID, second.GuardianProfileID)

	require.NoError(t, env.service.RejectInvitation(ctx, *first.InvitationID, approver))

	p, err := env.repos.GuardianProfile.FindByID(ctx, first.GuardianProfileID)
	require.NoError(t, err)
	assert.NotNil(t, p, "profile with another open invitation must survive reject cleanup")

	inv, err := env.repos.GuardianInvitation.FindByID(ctx, *second.InvitationID)
	require.NoError(t, err)
	assert.Equal(t, authModels.GuardianInvitationApprovalPending, inv.ApprovalStatus)
}
