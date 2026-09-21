package behavior_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// #3376, the Burbach case: offboarding a staff member disables the account
// when the school was its last one, and the account row keeps the address.
// The re-invitation the school sends afterwards used to find that row, ask
// the invitee to sign in to an account nobody can sign in to, and leave no
// way out — a password reset cannot clear the disabled flag either.
// Accepting the invitation now restores the account instead.
func TestInvitationRestoresOffboardedAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	factory := setupAuthFactory(t, db)
	invitations := &fixtureOwnedInvitationService{InvitationCapability: factory.Invitation, t: t, db: db}
	service := newFixtureAuthService(t, db, factory.Auth)
	access := factory.AccountAuthentication()

	email, username := uniqueTestCredentials("offboarded-headmaster")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	_, _, err = service.LoginWithAudit(context.Background(), email, testPassword, "", "", "")
	require.NoError(t, err, "the account has to work before it is offboarded")

	// ARRANGE — the school removes the user. This is the production path:
	// roles, permissions and sessions go, the mapping is deactivated, and
	// the account is disabled because this was its only school.
	preview, err := access.PreviewStaffOffboarding(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, preview.DeactivateAccount)
	result, err := access.ExecuteStaffOffboarding(ctx, account.ID, preview.Revision)
	require.NoError(t, err)
	require.True(t, result.AccountDeactivated)

	// ACT — the school adds the same address again.
	role := testpkg.CreateTestRoleForTenant(t, db, "restored-staff", tenantID)
	creator := testpkg.CreateTestAccount(t, db, "invitation-creator")
	invitation, err := invitations.CreateSchoolInvitation(testpkg.TenantContext(tenantID), identityaccess.SchoolInvitationRequest{
		Email: email, RoleID: role.ID, TenantID: tenantID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Offboarded"), LastName: testpkg.StrPtr("Headmaster"),
	})
	require.NoError(t, err)

	preview2, err := invitations.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	require.False(t, preview2.RequiresAccountLogin,
		"a dormant account has no login to offer, so the invitee sets a new password")

	accepted, err := invitations.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		FirstName: "Offboarded", LastName: "Headmaster",
		Password: testNewPassword, ConfirmPassword: testNewPassword,
	})
	require.NoError(t, err)
	require.Equal(t, account.ID, accepted.ID, "the invitee keeps the account the address belongs to")

	// ASSERT — the restored account signs in with the password just chosen,
	// and only with that one.
	accessToken, refreshToken, err := service.LoginWithAudit(context.Background(), email, testNewPassword, "", "", "")
	require.NoError(t, err)
	require.NotEmpty(t, accessToken)
	require.NotEmpty(t, refreshToken)
	_, _, err = service.LoginWithAudit(context.Background(), email, testPassword, "", "", "")
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials, "the old credential must not survive the restore")

	active, err := factory.Auth.VerifyAccountTenantMembership(ctx, account.ID, tenantID)
	require.NoError(t, err)
	require.True(t, active)
	roles, err := access.GetAccountRoles(ctx, account.ID)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	require.Equal(t, role.Name, roles[0].Name)
}

// An account disabled while it still holds school access was disabled on
// purpose. An invitation must not hand its credential to whoever received
// the mail; that account can still be signed in to, so the owner proof
// stays the only way in.
func TestInvitationLeavesDisabledAccountWithSchoolAccessAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	factory := setupAuthFactory(t, db)
	invitations := &fixtureOwnedInvitationService{InvitationCapability: factory.Invitation, t: t, db: db}
	service := newFixtureAuthService(t, db, factory.Auth)

	email, username := uniqueTestCredentials("disabled-member")
	account, err := service.Register(testpkg.TenantContext(tenantID), email, username, testPassword, nil, 0)
	require.NoError(t, err)
	testpkg.MapAccountToTenant(t, db, account.ID, tenantID)
	setAccountActive(t, db, account.ID, false)
	before, err := testpkg.ReadAccountState(context.Background(), db, account.ID)
	require.NoError(t, err)

	otherID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherID)
	role := testpkg.CreateTestRoleForTenant(t, db, "second-school-staff", otherID)
	creator := testpkg.CreateTestAccount(t, db, "invitation-creator")
	invitation, err := invitations.CreateSchoolInvitation(testpkg.TenantContext(otherID), identityaccess.SchoolInvitationRequest{
		Email: email, RoleID: role.ID, TenantID: otherID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Disabled"), LastName: testpkg.StrPtr("Member"),
	})
	require.NoError(t, err)

	preview, err := invitations.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	require.True(t, preview.RequiresAccountLogin)

	_, err = invitations.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		FirstName: "Disabled", LastName: "Member",
		Password: testNewPassword, ConfirmPassword: testNewPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationOwnerRequired)

	stored, err := testpkg.ReadAccountState(context.Background(), db, account.ID)
	require.NoError(t, err)
	require.False(t, stored.Active, "the deliberate deactivation stands")
	require.Equal(t, before.PasswordHash, stored.PasswordHash, "the invitation must not replace the credential")
}
