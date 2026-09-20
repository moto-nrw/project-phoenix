package application

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bulk invitation (#3378) counts guardians, never children, and never
// widens anybody's access: a restrictive contact stays restrictive, an open
// invitation is only touched on request, and a dry run writes nothing.

const roleLegal = "legal_guardian"

func (f *lifecycleFixture) bulkGuardian(email, role string, studentIDs ...int64) domain.GuardianProfile {
	profile := f.guardians.addProfile(domain.GuardianProfile{TenantID: lifecycleTenant, FirstName: "Eltern", LastName: email, Email: email})
	for _, studentID := range studentIDs {
		f.guardians.addLink(domain.StudentGuardianLink{
			TenantID: lifecycleTenant, StudentID: studentID, GuardianProfileID: profile.ID, GuardianRole: role,
		})
	}
	return profile
}

func (f *lifecycleFixture) openInvitation(profileID, studentID int64, status string, expires time.Time) domain.GuardianInvitation {
	return f.invitations.add(domain.GuardianInvitation{
		TenantID: lifecycleTenant, GuardianProfileID: profileID, StudentID: &studentID,
		ApprovalStatus: status, ExpiresAt: expires, Token: "open-token",
	})
}

func bulkRequest(studentIDs ...int64) domain.BulkInviteRequest {
	return domain.BulkInviteRequest{StudentIDs: studentIDs, CreatedBy: relativeActor}
}

func TestBulkInvite_ValidatesTheRequest(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)

	for name, req := range map[string]domain.BulkInviteRequest{
		"created by":  {StudentIDs: []int64{1}},
		"no students": {CreatedBy: relativeActor},
		"negative id": {CreatedBy: relativeActor, StudentIDs: []int64{1, -2}},
		"too many":    {CreatedBy: relativeActor, StudentIDs: make([]int64, domain.BulkInviteMaxStudents+1)},
	} {
		_, err := f.lifecycle.BulkInviteToStudents(tenantContext(), req)
		require.Error(t, err, name)
		var validation *domain.GuardianInvitationValidationError
		require.ErrorAs(t, err, &validation, name)
	}
}

func TestBulkInvite_OneMailPerGuardianAcrossSiblings(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	parent := f.bulkGuardian("mutter@example.test", roleLegal, 11, 12, 13)

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(13, 11, 12))

	require.NoError(t, err)
	assert.Equal(t, 1, result.Invited)
	require.Len(t, f.delivery.emails, 1, "three children, one guardian, one mail")
	assert.Equal(t, parent.ID, f.delivery.emails[0].GuardianProfileID)
	require.NotNil(t, f.delivery.emails[0].StudentID)
	assert.Equal(t, int64(11), *f.delivery.emails[0].StudentID, "anchored to the first selected child")
	assert.Len(t, f.guardians.links, 3, "no relationship is created or changed")
}

func TestBulkInvite_SkipsActiveRestrictedAndOpen(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	accountID := int64(40)
	active := f.bulkGuardian("aktiv@example.test", roleLegal, 11)
	active.AccountID = &accountID
	active.HasAccount = true
	f.guardians.profiles[active.ID] = active
	f.bulkGuardian("oma@example.test", "pickup_only", 11)
	f.bulkGuardian("sozial@example.test", "social_worker", 11)
	waiting := f.bulkGuardian("offen@example.test", roleLegal, 12)
	f.openInvitation(waiting.ID, 12, domain.GuardianInvitationApprovalNotRequired, time.Now().Add(time.Hour))

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11, 12))

	require.NoError(t, err)
	assert.Equal(t, domain.BulkInviteResult{SkippedActive: 1, SkippedRestricted: 2, SkippedOpen: 1, Problems: []domain.BulkInviteProblem{}}, *result)
	assert.Empty(t, f.delivery.emails)
	assert.Empty(t, f.guardians.promoted, "a bulk run never upgrades a restrictive contact")
}

func TestBulkInvite_RecordsAnActiveProfileEmailBeforeSkippingIt(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	accountID := int64(40)
	active := f.bulkGuardian("geteilt@example.test", roleLegal, 11)
	active.AccountID = &accountID
	active.HasAccount = true
	f.guardians.profiles[active.ID] = active
	f.bulkGuardian("geteilt@example.test", roleLegal, 12)

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11, 12))

	require.NoError(t, err)
	assert.Equal(t, 1, result.SkippedActive)
	require.Len(t, result.Problems, 1)
	assert.Equal(t, domain.BulkInviteProblemDuplicateEmail, result.Problems[0].Reason)
	assert.Zero(t, result.Invited)
	assert.Empty(t, f.delivery.emails)
}

func TestBulkInvite_RegrantsTenantAccessForAnActiveGuardian(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	accountID := int64(40)
	active := f.bulkGuardian("aktiv@example.test", roleLegal, 11)
	active.AccountID = &accountID
	active.HasAccount = true
	f.guardians.profiles[active.ID] = active

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11))

	require.NoError(t, err)
	assert.Equal(t, 1, result.SkippedActive)
	assert.Equal(t, []int64{accountID}, f.store.granted,
		"an active profile regains its tenant access when a previous mapping was removed")
}

func TestBulkInvite_LocksProfilesBeforeReadingOpenInvitations(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	parent := f.bulkGuardian("mutter@example.test", roleLegal, 11)
	f.runtime.lockHook = func(string) {
		f.openInvitation(parent.ID, 11, domain.GuardianInvitationApprovalNotRequired, time.Now().Add(time.Hour))
	}

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11))

	require.NoError(t, err)
	assert.Equal(t, 1, result.SkippedOpen)
	assert.Empty(t, f.delivery.emails)
	assert.Equal(t, []string{fmt.Sprintf("guardian-bulk-invite:77:%d", parent.ID)}, f.runtime.locks)
}

func TestBulkInvite_BatchesExistingAccountLookups(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.bulkGuardian("eins@example.test", roleLegal, 11)
	f.bulkGuardian("zwei@example.test", roleLegal, 12)
	f.bulkGuardian("drei@example.test", roleLegal, 13)

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11, 12, 13))

	require.NoError(t, err)
	assert.Equal(t, 3, result.Invited)
	assert.Equal(t, 1, f.store.findAccountsByEmailsCalls)
}

func TestBulkInvite_ResendRestartsTheWindow(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	waiting := f.bulkGuardian("offen@example.test", roleLegal, 12)
	almostExpired := time.Now().Add(10 * time.Minute)
	open := f.openInvitation(waiting.ID, 12, domain.GuardianInvitationApprovalNotRequired, almostExpired)

	req := bulkRequest(12)
	req.ResendOpen = true
	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), req)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Resent)
	assert.Zero(t, result.Invited)
	require.Len(t, f.delivery.emails, 1)
	assert.Equal(t, open.ID, f.delivery.emails[0].ID, "the open invitation is reused, not duplicated")
	assert.True(t, f.delivery.emails[0].ExpiresAt.After(time.Now().Add(47*time.Hour)), "nobody gets a link that is about to expire")
}

func TestBulkInvite_ResendSupersedesAParentRequestAwaitingApproval(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	requested := f.bulkGuardian("angefragt@example.test", roleLegal, 12)
	pending := f.openInvitation(requested.ID, 12, domain.GuardianInvitationApprovalPending, time.Now().Add(time.Hour))

	req := bulkRequest(12)
	req.ResendOpen = true
	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), req)

	require.NoError(t, err)
	assert.Equal(t, 1, result.Invited)
	require.Len(t, f.delivery.emails, 1)
	assert.Equal(t, pending.ID, f.delivery.emails[0].ID)
	assert.Equal(t, domain.GuardianInvitationApprovalNotRequired, f.delivery.emails[0].ApprovalStatus)
}

func TestBulkInvite_ReportsAddressProblemsWithTheChild(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.guardians.students[11] = domain.Student{ID: 11, PersonID: 111}
	f.guardians.names[111] = domain.PersonName{FirstName: "Mia", LastName: "Brenner"}
	missing := f.bulkGuardian("", roleLegal, 11)
	f.bulkGuardian("kein-at-zeichen", roleLegal, 11)
	f.bulkGuardian("familie@example.test", roleLegal, 11)
	f.bulkGuardian("Familie@example.test ", roleLegal, 11)

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11))

	require.NoError(t, err)
	assert.Equal(t, 1, result.Invited, "a shared address is mailed once")
	require.Len(t, result.Problems, 3)
	assert.Equal(t, domain.BulkInviteProblem{
		GuardianProfileID: missing.ID, GuardianName: missing.FullName(), StudentNames: []string{"Mia Brenner"},
		Reason: domain.BulkInviteProblemMissingEmail,
	}, result.Problems[0])
	assert.Equal(t, domain.BulkInviteProblemInvalidEmail, result.Problems[1].Reason)
	assert.Equal(t, domain.BulkInviteProblemDuplicateEmail, result.Problems[2].Reason)
}

func TestBulkInvite_ExistingAccountIsLinkedAndToldWhereToLogIn(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.addAccount(40, "konto@example.test", "hash:secret", true)
	parent := f.bulkGuardian("konto@example.test", roleLegal, 11)

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11))

	require.NoError(t, err)
	assert.Equal(t, 1, result.LinkedExistingAccount)
	assert.Zero(t, result.Invited)
	assert.Empty(t, f.delivery.emails, "no token for somebody who can already log in")
	require.Len(t, f.delivery.accessEmails, 1)
	assert.Equal(t, parent.ID, f.delivery.accessEmails[0].ID)
}

func TestBulkInvite_ExistingAccountWinsOverAnOpenInvitation(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.addAccount(40, "konto@example.test", "hash:secret", true)
	parent := f.bulkGuardian("konto@example.test", roleLegal, 11)
	open := f.openInvitation(parent.ID, 11, domain.GuardianInvitationApprovalNotRequired, time.Now().Add(time.Hour))
	req := bulkRequest(11)
	req.ResendOpen = true

	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), req)

	require.NoError(t, err)
	assert.Equal(t, 1, result.LinkedExistingAccount)
	assert.Zero(t, result.Resent)
	assert.Empty(t, f.delivery.emails, "an account holder must not receive another token")
	require.Len(t, f.delivery.accessEmails, 1)
	assert.Equal(t, parent.ID, f.delivery.accessEmails[0].ID)
	assert.Equal(t, open.ExpiresAt, f.invitations.rows[open.ID].ExpiresAt, "the token is not resent")
}

func TestBulkInvite_DryRunWritesAndMailsNothing(t *testing.T) {
	t.Parallel()
	f := newLifecycleFixture(t)
	f.store.addAccount(40, "konto@example.test", "hash:secret", true)
	f.bulkGuardian("konto@example.test", roleLegal, 11)
	f.bulkGuardian("neu@example.test", roleLegal, 11)
	waiting := f.bulkGuardian("offen@example.test", roleLegal, 12)
	expires := time.Now().Add(10 * time.Minute)
	f.openInvitation(waiting.ID, 12, domain.GuardianInvitationApprovalNotRequired, expires)

	req := bulkRequest(11, 12)
	req.DryRun, req.ResendOpen = true, true
	result, err := f.lifecycle.BulkInviteToStudents(tenantContext(), req)

	require.NoError(t, err)
	assert.True(t, result.DryRun)
	assert.Equal(t, 2, result.Invited)
	assert.Equal(t, 1, result.Resent)
	assert.Empty(t, f.delivery.emails)
	assert.Empty(t, f.delivery.accessEmails)
	require.Len(t, f.invitations.rows, 1, "no invitation is created")
	for _, invitation := range f.invitations.rows {
		assert.WithinDuration(t, expires, invitation.ExpiresAt, time.Second, "the open invitation is untouched")
	}
	for _, profile := range f.guardians.profiles {
		assert.False(t, profile.HasAccount, "no account is attached")
	}
}

func TestBulkInvite_RepoErrorsSurface(t *testing.T) {
	t.Parallel()

	t.Run("links", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.guardians.listByStudentErr = errLifecycleBoom
		_, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11))
		require.ErrorIs(t, err, errLifecycleBoom)
	})

	t.Run("invitation insert", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.bulkGuardian("neu@example.test", roleLegal, 11)
		f.invitations.insertErr = errLifecycleBoom
		_, err := f.lifecycle.BulkInviteToStudents(tenantContext(), bulkRequest(11))
		require.ErrorIs(t, err, errLifecycleBoom)
	})
}
