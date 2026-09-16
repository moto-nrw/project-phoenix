package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Guardian relative access moved out of services/auth (#3225). Parent portal
// authority is relationship-scoped: the link's guardian role decides, never
// the school membership alone. The DB-backed behaviour suite still runs in
// services/auth through the composed module; these cases pin the error paths
// and the role decisions at the application seam.

const (
	relativeStudent int64 = 5
	relativeActor   int64 = 7
)

func inviteRequest(email string) domain.InviteToStudentRequest {
	return domain.InviteToStudentRequest{StudentID: relativeStudent, Email: email, CreatedBy: relativeActor}
}

func pendingApproval(f *lifecycleFixture, profileID int64, created bool) domain.GuardianInvitation {
	studentID := relativeStudent
	return f.invitations.add(domain.GuardianInvitation{
		TenantID: lifecycleTenant, GuardianProfileID: profileID, StudentID: &studentID,
		ApprovalStatus: domain.GuardianInvitationApprovalPending, ProfileCreatedForInvitation: created,
		ExpiresAt: time.Now().Add(time.Hour),
	})
}

func TestInviteToStudent_ValidatesTheRequest(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	for name, req := range map[string]domain.InviteToStudentRequest{
		"student":    {Email: "a@b.de", CreatedBy: relativeActor},
		"created by": {StudentID: relativeStudent, Email: "a@b.de"},
		"email":      {StudentID: relativeStudent, CreatedBy: relativeActor, Email: "  "},
	} {
		_, err := f.lifecycle.InviteToStudent(tenantContext(), req)
		require.Error(t, err, name)
	}
	assert.Empty(t, f.guardians.profiles)
}

func TestInviteToStudent_RepoErrorsSurface(t *testing.T) {
	t.Parallel()

	t.Run("profile create fails", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.guardians.createProfileErr = errLifecycleBoom
		_, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("a@b.de"))
		require.ErrorIs(t, err, errLifecycleBoom)
	})

	t.Run("link fails", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.guardians.linkErr = errLifecycleBoom
		_, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("a@b.de"))
		require.ErrorIs(t, err, errLifecycleBoom)
	})

	t.Run("invitation create fails", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.invitations.insertErr = errLifecycleBoom
		_, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("a@b.de"))
		require.ErrorIs(t, err, errLifecycleBoom)
		var operation *OperationError
		require.ErrorAs(t, err, &operation)
		assert.Equal(t, "invite guardian to student", operation.Op)
	})
}

func TestInviteToStudent_NewEmailCreatesProfileLinkAndInvitation(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	before := time.Now()

	result, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest(" New@Example.test "))
	require.NoError(t, err)

	assert.Equal(t, domain.InviteOutcomeInvited, result.Outcome)
	require.NotNil(t, result.InvitationID)
	profile := f.guardians.profiles[result.GuardianProfileID]
	assert.Equal(t, "new@example.test", profile.Email, "the address is normalized")
	_, linked := f.guardians.linkOf(relativeStudent, profile.ID)
	assert.True(t, linked)
	invitation := f.invitations.rows[*result.InvitationID]
	assert.Equal(t, domain.GuardianInvitationApprovalNotRequired, invitation.ApprovalStatus)
	assert.True(t, invitation.ProfileCreatedForInvitation)
	assert.WithinDuration(t, before.Add(48*time.Hour), invitation.ExpiresAt, 5*time.Second)
	require.Len(t, f.delivery.emails, 1, "the invitation e-mail is enqueued")
}

func TestInviteToStudent_ExistingAccountIsLinkedWithoutInvitation(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.addAccount(40, "parent@example.test", "hash:secret", true)

	result, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("parent@example.test"))
	require.NoError(t, err)
	assert.Equal(t, domain.InviteOutcomeLinkedExistingAccount, result.Outcome)
	assert.Nil(t, result.InvitationID)
	profile := f.guardians.profiles[result.GuardianProfileID]
	require.NotNil(t, profile.AccountID)
	assert.Equal(t, int64(40), *profile.AccountID)
	assert.Equal(t, []int64{40}, f.store.granted, "the guardian role is granted at the school")
	assert.Empty(t, f.delivery.emails)

	again, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("parent@example.test"))
	require.NoError(t, err)
	assert.Equal(t, domain.InviteOutcomeAlreadyLinked, again.Outcome)
}

func TestInviteToStudent_ApprovalModeQueuesWithoutLinking(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.addAccount(40, "parent@example.test", "hash:secret", true)
	req := inviteRequest("parent@example.test")
	requester := relativeActor
	req.RequestedByParentAccountID = &requester
	req.RequireApproval = true

	result, err := f.lifecycle.InviteToStudent(tenantContext(), req)
	require.NoError(t, err)
	assert.Equal(t, domain.InviteOutcomePendingApproval, result.Outcome)
	_, linked := f.guardians.linkOf(relativeStudent, result.GuardianProfileID)
	assert.False(t, linked, "staff approval must not create the child link")
	assert.False(t, f.guardians.profiles[result.GuardianProfileID].HasAccount, "no account is attached before approval")
	assert.Empty(t, f.store.granted)
	assert.Empty(t, f.delivery.emails)
}

// An existing restrictive contact must not silently stay without portal
// access (#2172): the invite asks first and changes nothing.
func TestInviteToStudent_RestrictedContactRequiresConfirmation(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	profile := f.guardians.addProfile(domain.GuardianProfile{Email: "pickup@example.test"})
	link := f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID, GuardianRole: "pickup_only"})

	result, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("pickup@example.test"))
	require.NoError(t, err)
	assert.Equal(t, domain.InviteOutcomeExistingContactRestricted, result.Outcome)
	assert.Equal(t, "pickup_only", result.ExistingRole)
	assert.Equal(t, "pickup_only", f.guardians.links[link.ID].GuardianRole)
	assert.Empty(t, f.invitations.rows)

	req := inviteRequest("pickup@example.test")
	req.ConfirmRoleUpgrade = true
	result, err = f.lifecycle.InviteToStudent(tenantContext(), req)
	require.NoError(t, err)
	assert.Equal(t, domain.InviteOutcomeInvited, result.Outcome)
	assert.Equal(t, []int64{link.ID}, f.guardians.promoted, "a confirmed staff upgrade applies at once")
}

func TestInviteToStudent_SocialWorkerIsNeverUpgraded(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	profile := f.guardians.addProfile(domain.GuardianProfile{Email: "sw@example.test"})
	f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID, GuardianRole: "social_worker"})

	req := inviteRequest("sw@example.test")
	req.ConfirmRoleUpgrade = true
	_, err := f.lifecycle.InviteToStudent(tenantContext(), req)
	require.ErrorIs(t, err, domain.ErrInviteSocialWorkerManaged)
	assert.Empty(t, f.guardians.promoted)
	assert.Empty(t, f.invitations.rows)
}

// A parent asking to upgrade a staff-set restriction always goes through
// staff approval, even in direct invite mode.
func TestInviteToStudent_ParentConfirmedUpgradeIsQueued(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	profile := f.guardians.addProfile(domain.GuardianProfile{Email: "pickup@example.test"})
	f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID, GuardianRole: "emergency_contact"})
	req := inviteRequest("pickup@example.test")
	requester := relativeActor
	req.RequestedByParentAccountID = &requester
	req.ConfirmRoleUpgrade = true

	result, err := f.lifecycle.InviteToStudent(tenantContext(), req)
	require.NoError(t, err)
	assert.Equal(t, domain.InviteOutcomePendingApproval, result.Outcome)
	assert.True(t, f.invitations.rows[*result.InvitationID].RoleUpgrade)
	assert.Empty(t, f.guardians.promoted, "nothing is upgraded before staff approve")
}

func TestApproveInvitation_Paths(t *testing.T) {
	t.Parallel()

	t.Run("profile lookup error surfaces", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		invitation := pendingApproval(f, 10, true)
		f.guardians.findProfileErr = errLifecycleBoom
		require.ErrorIs(t, f.lifecycle.ApproveInvitation(tenantContext(), invitation.ID, relativeActor), errLifecycleBoom)
	})

	t.Run("not found, not pending and expired are refused", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		require.ErrorIs(t, f.lifecycle.ApproveInvitation(tenantContext(), 404, relativeActor), domain.ErrGuardianInvitationNotFound)

		decided := pendingApproval(f, 10, false)
		decided.ApprovalStatus = domain.GuardianInvitationApprovalApproved
		f.invitations.rows[decided.ID] = decided
		require.Error(t, f.lifecycle.ApproveInvitation(tenantContext(), decided.ID, relativeActor))

		expired := pendingApproval(f, 10, false)
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		f.invitations.rows[expired.ID] = expired
		require.ErrorIs(t, f.lifecycle.ApproveInvitation(tenantContext(), expired.ID, relativeActor), domain.ErrGuardianInvitationExpired)
		require.ErrorIs(t, f.lifecycle.RejectInvitation(tenantContext(), expired.ID, relativeActor), domain.ErrGuardianInvitationExpired)
	})

	t.Run("no account links the child and sends the invitation", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "new@example.test"})
		invitation := pendingApproval(f, profile.ID, true)

		require.NoError(t, f.lifecycle.ApproveInvitation(tenantContext(), invitation.ID, relativeActor))
		stored := f.invitations.rows[invitation.ID]
		assert.Equal(t, domain.GuardianInvitationApprovalApproved, stored.ApprovalStatus)
		require.NotNil(t, stored.ApprovedBy)
		assert.Equal(t, relativeActor, *stored.ApprovedBy)
		assert.Nil(t, stored.AcceptedAt)
		_, linked := f.guardians.linkOf(relativeStudent, profile.ID)
		assert.True(t, linked)
		assert.Len(t, f.delivery.emails, 1)
	})

	t.Run("an existing account is attached and the invitation closed", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.store.addAccount(40, "parent@example.test", "hash:secret", true)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "Parent@example.test"})
		invitation := pendingApproval(f, profile.ID, false)

		require.NoError(t, f.lifecycle.ApproveInvitation(tenantContext(), invitation.ID, relativeActor))
		assert.NotNil(t, f.invitations.rows[invitation.ID].AcceptedAt)
		assert.True(t, f.guardians.profiles[profile.ID].HasAccount)
		assert.Empty(t, f.delivery.emails)
	})

	// A promised upgrade that can no longer be applied aborts the approval:
	// the request stays pending and no e-mail promises access.
	t.Run("upgrade of a social-worker link aborts", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "sw@example.test"})
		f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID, GuardianRole: "social_worker"})
		invitation := pendingApproval(f, profile.ID, false)
		invitation.RoleUpgrade = true
		f.invitations.rows[invitation.ID] = invitation

		err := f.lifecycle.ApproveInvitation(tenantContext(), invitation.ID, relativeActor)
		require.ErrorIs(t, err, domain.ErrInviteSocialWorkerManaged)
		assert.True(t, f.invitations.rows[invitation.ID].IsPendingApproval())
		assert.Empty(t, f.delivery.emails)
	})
}

func TestPendingInvitationStudentID(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	invitation := pendingApproval(f, 10, false)

	studentID, err := f.lifecycle.PendingInvitationStudentID(tenantContext(), invitation.ID)
	require.NoError(t, err)
	assert.Equal(t, relativeStudent, studentID)

	invitation.StudentID = nil
	f.invitations.rows[invitation.ID] = invitation
	_, err = f.lifecycle.PendingInvitationStudentID(tenantContext(), invitation.ID)
	require.Error(t, err)
}

func TestRejectInvitation_UpdateErrorSurfaces(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	invitation := pendingApproval(f, 10, true)
	f.invitations.updateErr = errLifecycleBoom
	require.ErrorIs(t, f.lifecycle.RejectInvitation(tenantContext(), invitation.ID, relativeActor), errLifecycleBoom)
}

func TestRejectInvitation_CleanupBranchesAreBestEffort(t *testing.T) {
	t.Parallel()

	t.Run("orphan profile is removed", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "orphan@example.test"})
		invitation := pendingApproval(f, profile.ID, true)
		require.NoError(t, f.lifecycle.RejectInvitation(tenantContext(), invitation.ID, relativeActor))
		assert.Equal(t, domain.GuardianInvitationApprovalRejected, f.invitations.rows[invitation.ID].ApprovalStatus)
		assert.Equal(t, []int64{profile.ID}, f.guardians.deletedProfiles)
	})

	t.Run("link check error is swallowed", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "orphan@example.test"})
		invitation := pendingApproval(f, profile.ID, true)
		f.guardians.listByProfileErr = errLifecycleBoom
		require.NoError(t, f.lifecycle.RejectInvitation(tenantContext(), invitation.ID, relativeActor))
		assert.Empty(t, f.guardians.deletedProfiles)
	})

	t.Run("delete error is swallowed", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "orphan@example.test"})
		invitation := pendingApproval(f, profile.ID, true)
		f.guardians.deleteProfileErr = errLifecycleBoom
		require.NoError(t, f.lifecycle.RejectInvitation(tenantContext(), invitation.ID, relativeActor))
	})

	t.Run("profile with account, other links or open invitations stays", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		accountID := int64(40)
		withAccount := f.guardians.addProfile(domain.GuardianProfile{Email: "a@example.test", AccountID: &accountID, HasAccount: true})
		linked := f.guardians.addProfile(domain.GuardianProfile{Email: "b@example.test"})
		f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent + 1, GuardianProfileID: linked.ID})
		shared := f.guardians.addProfile(domain.GuardianProfile{Email: "c@example.test"})
		pendingApproval(f, shared.ID, false)

		for _, profile := range []domain.GuardianProfile{withAccount, linked, shared} {
			invitation := pendingApproval(f, profile.ID, true)
			require.NoError(t, f.lifecycle.RejectInvitation(tenantContext(), invitation.ID, relativeActor))
		}
		assert.Empty(t, f.guardians.deletedProfiles)
	})
}

func TestListPendingApprovalsDetailed(t *testing.T) {
	t.Parallel()

	t.Run("queue error surfaces", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.invitations.pendingErr = errLifecycleBoom
		_, err := f.lifecycle.ListPendingApprovalsDetailed(tenantContext())
		require.ErrorIs(t, err, errLifecycleBoom)
	})

	t.Run("empty queue is an empty list", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		views, err := f.lifecycle.ListPendingApprovalsDetailed(tenantContext())
		require.NoError(t, err)
		assert.NotNil(t, views)
		assert.Empty(t, views)
	})

	t.Run("names are resolved", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.store.addAccount(relativeActor, "requester@example.test", "hash:secret", true)
		profile := f.guardians.addProfile(domain.GuardianProfile{FirstName: "Gerda", LastName: "Gast", Email: " gerda@example.test "})
		f.guardians.students[relativeStudent] = domain.Student{ID: relativeStudent, PersonID: 300}
		f.guardians.names[300] = domain.PersonName{FirstName: "Kim", LastName: "Kind"}
		invitation := pendingApproval(f, profile.ID, false)
		requester := relativeActor
		invitation.RequestedByAccountID = &requester
		invitation.RoleUpgrade = true
		f.invitations.rows[invitation.ID] = invitation

		views, err := f.lifecycle.ListPendingApprovalsDetailed(tenantContext())
		require.NoError(t, err)
		require.Len(t, views, 1)
		assert.Equal(t, "Gerda Gast", views[0].GuardianName)
		assert.Equal(t, "gerda@example.test", views[0].GuardianEmail)
		assert.Equal(t, "Kim Kind", views[0].StudentName)
		assert.Equal(t, "requester@example.test", views[0].RequestedByEmail)
		assert.True(t, views[0].RoleUpgrade)
	})

	t.Run("name resolution is best effort", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		invitation := pendingApproval(f, 10, false)
		requester := relativeActor
		invitation.RequestedByAccountID = &requester
		f.invitations.rows[invitation.ID] = invitation
		f.guardians.findProfilesErr = errLifecycleBoom
		f.guardians.findStudentsErr = errLifecycleBoom

		views, err := f.lifecycle.ListPendingApprovalsDetailed(tenantContext())
		require.NoError(t, err)
		require.Len(t, views, 1)
		assert.Empty(t, views[0].GuardianName)
		assert.Empty(t, views[0].StudentName)
		assert.Empty(t, views[0].RequestedByEmail)
	})
}

func TestRevokeAccess(t *testing.T) {
	t.Parallel()

	t.Run("link lookup error surfaces", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.guardians.listByStudentErr = errLifecycleBoom
		err := f.lifecycle.RevokeAccess(tenantContext(), domain.RevokeAccessRequest{StudentID: relativeStudent, GuardianProfileID: 10, ActorAccountID: relativeActor})
		require.ErrorIs(t, err, errLifecycleBoom)
		assert.Equal(t, []int64{relativeStudent}, f.guardians.locked, "the child's row is locked first")
	})

	t.Run("validation and unlinked", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		require.Error(t, f.lifecycle.RevokeAccess(tenantContext(), domain.RevokeAccessRequest{GuardianProfileID: 10}))
		require.Error(t, f.lifecycle.RevokeAccess(tenantContext(), domain.RevokeAccessRequest{StudentID: relativeStudent, GuardianProfileID: 10}))
	})

	t.Run("a parent cannot remove the primary guardian, staff can", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "primary@example.test"})
		link := f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID, IsPrimary: true})
		req := domain.RevokeAccessRequest{StudentID: relativeStudent, GuardianProfileID: profile.ID, ActorAccountID: relativeActor, ByParent: true}

		require.ErrorIs(t, f.lifecycle.RevokeAccess(tenantContext(), req), domain.ErrCannotRemovePrimaryGuardian)
		req.ByParent = false
		require.NoError(t, f.lifecycle.RevokeAccess(tenantContext(), req))
		assert.Equal(t, []int64{link.ID}, f.guardians.deletedLinks)
	})

	t.Run("the payer stays without the financial permission", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "payer@example.test"})
		link := f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID, IsPayer: true})
		req := domain.RevokeAccessRequest{StudentID: relativeStudent, GuardianProfileID: profile.ID, ActorAccountID: relativeActor}

		require.ErrorIs(t, f.lifecycle.RevokeAccess(tenantContext(), req), domain.ErrCannotRemovePayerGuardian)
		assert.Empty(t, f.guardians.deletedLinks)

		f.financial.err = errLifecycleBoom
		req.MayClearPayer = true
		require.ErrorIs(t, f.lifecycle.RevokeAccess(tenantContext(), req), errLifecycleBoom, "no removal without the financial audit")
		assert.Empty(t, f.guardians.deletedLinks)

		f.financial.err = nil
		require.NoError(t, f.lifecycle.RevokeAccess(tenantContext(), req))
		assert.Equal(t, []int64{profile.ID}, f.financial.removed)
		assert.Equal(t, []int64{link.ID}, f.guardians.deletedLinks)
	})

	t.Run("a parent cannot remove their own access", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		actor := relativeActor
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "self@example.test", AccountID: &actor, HasAccount: true})
		f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID})
		err := f.lifecycle.RevokeAccess(tenantContext(), domain.RevokeAccessRequest{
			StudentID: relativeStudent, GuardianProfileID: profile.ID, ActorAccountID: relativeActor, ByParent: true,
		})
		require.ErrorIs(t, err, domain.ErrCannotRemoveOwnAccess)
	})

	t.Run("a parent cannot remove a staff-managed contact", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "staff@example.test"})
		f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID})
		err := f.lifecycle.RevokeAccess(tenantContext(), domain.RevokeAccessRequest{
			StudentID: relativeStudent, GuardianProfileID: profile.ID, ActorAccountID: relativeActor, ByParent: true,
		})
		require.ErrorIs(t, err, domain.ErrCannotRemoveStaffManagedGuardian)
		assert.Empty(t, f.guardians.deletedLinks)
	})

	// A parent cancelling a pending invite for a contact the school keeps
	// expires the invitation but leaves the staff-managed link in place.
	t.Run("a parent cancels a pending invite without deleting the contact", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		profile := f.guardians.addProfile(domain.GuardianProfile{Email: "staff@example.test"})
		f.guardians.addLink(domain.StudentGuardianLink{StudentID: relativeStudent, GuardianProfileID: profile.ID})
		invitation := pendingApproval(f, profile.ID, false)

		require.NoError(t, f.lifecycle.RevokeAccess(tenantContext(), domain.RevokeAccessRequest{
			StudentID: relativeStudent, GuardianProfileID: profile.ID, ActorAccountID: relativeActor, ByParent: true,
		}))
		stored := f.invitations.rows[invitation.ID]
		assert.Equal(t, domain.GuardianInvitationApprovalRejected, stored.ApprovalStatus)
		assert.False(t, stored.ExpiresAt.After(time.Now()))
		assert.Empty(t, f.guardians.deletedLinks)
	})
}

func TestRelativeAccessRunsOnTheCallersTenant(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	_, err := f.lifecycle.InviteToStudent(context.Background(), inviteRequest("tenantless@example.test"))
	require.NoError(t, err)
	for _, profile := range f.guardians.profiles {
		assert.Zero(t, profile.TenantID, "a tenantless caller creates no tenant-stamped row")
	}

	result, err := f.lifecycle.InviteToStudent(tenantContext(), inviteRequest("tenant@example.test"))
	require.NoError(t, err)
	assert.Equal(t, lifecycleTenant, f.guardians.profiles[result.GuardianProfileID].TenantID)
	assert.Equal(t, lifecycleTenant, f.invitations.rows[*result.InvitationID].TenantID)
}
