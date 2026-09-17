package auth_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The school invitation flows live in Identity & Access (#2722); the tests
// below drive them through the retained invitation service the way the
// invitation routes and the staff import do, against a real database.

const (
	invitationPassword = "Str0ngP@ssword!" //nolint:gosec // test-only value, never a real credential
	// The permission names the invitation routes pass from the caller's JWT.
	usersManagePermission = "users:manage"
	usersCreatePermission = "users:create"
)

type invitationEnv struct {
	service authService.InvitationService
	repos   *repositories.InvitationPersistence
	db      *bun.DB
}

func newInvitationEnv(t *testing.T, db *bun.DB) invitationEnv {
	t.Helper()
	repos, err := repositories.NewInvitationPersistence(db)
	require.NoError(t, err)
	return invitationEnv{service: setupInvitationService(t, db), repos: repos, db: db}
}

func inviteeAddress(prefix string) string {
	return fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano())
}

// storedInvitation reads the row itself, so a test can look at an
// invitation of another school than the one in context.
func (e invitationEnv) storedInvitation(t *testing.T, id int64) invitationRow {
	t.Helper()
	var row invitationRow
	require.NoError(t, e.db.NewRaw(`SELECT used_at, expires_at, email_error, email_sent_at FROM auth.invitation_tokens WHERE id = ?`, id).
		Scan(context.Background(), &row))
	return row
}

type invitationRow struct {
	UsedAt      *time.Time `bun:"used_at"`
	ExpiresAt   time.Time  `bun:"expires_at"`
	EmailError  *string    `bun:"email_error"`
	EmailSentAt *time.Time `bun:"email_sent_at"`
}

func (r invitationRow) IsUsed() bool { return r.UsedAt != nil }

// An invitation is stored only while it can be redeemed, hands the invitee
// the school access it promises, and is spent exactly once.
func TestInvitationIsRedeemableOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-creator")
	role := testpkg.CreateTestRole(t, db, "invited-staff")
	address := inviteeAddress("invitee")

	invitation, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: " " + address + " ", RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	require.NotNil(t, invitation)
	assert.Equal(t, address, invitation.Email, "the address is normalized")
	assert.NotEmpty(t, invitation.Token)
	assert.True(t, invitation.ExpiresAt.After(time.Now()), "an invitation is never stored already expired")
	assert.Equal(t, testpkg.Tenant(t), invitation.TenantID)

	preview, err := env.service.ValidateInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, "tenant", preview.TargetPortal)
	assert.False(t, preview.RequiresAccountLogin, "an unknown address signs up instead of signing in")
	assert.Equal(t, address, preview.Email)
	assert.Equal(t, role.Name, preview.RoleName)

	account, err := env.service.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	testpkg.OwnTestAccount(t, db, account.ID)
	assert.Equal(t, address, account.Email)

	member, err := env.repos.AccountTenant.ExistsByAccountAndTenant(context.Background(), account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.True(t, member, "the invitee can sign in at the school")
	person, err := env.repos.Person.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, person, "accepting provisions the identity the role needs")
	assert.Equal(t, "Ada", person.FirstName)
	assert.True(t, env.storedInvitation(t, invitation.ID).IsUsed())

	_, err = env.service.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, authService.ErrInvitationUsed, "a spent invitation grants no second access")
	_, err = env.service.ValidateInvitation(context.Background(), invitation.Token)
	require.ErrorIs(t, err, authService.ErrInvitationUsed)
	_, err = env.service.ValidateInvitation(context.Background(), "unknown-token")
	require.ErrorIs(t, err, authService.ErrInvitationNotFound)
}

// A second invitation for the same address spends the first, so only the
// newest link in an inbox works — and only inside its own school.
func TestCreateInvitationInvalidatesThePreviousInvitationOfTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-replacing-creator")
	role := testpkg.CreateTestRole(t, db, "invited-replaced")
	address := inviteeAddress("replaced")
	request := authService.InvitationRequest{
		Email: address, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	}

	first, err := env.service.CreateInvitation(ctx, request)
	require.NoError(t, err)
	second, err := env.service.CreateInvitation(ctx, request)
	require.NoError(t, err)

	assert.True(t, env.storedInvitation(t, first.ID).IsUsed(), "the previous invitation is spent")
	_, err = env.service.ValidateInvitation(context.Background(), first.Token)
	require.ErrorIs(t, err, authService.ErrInvitationUsed)
	_, err = env.service.ValidateInvitation(context.Background(), second.Token)
	require.NoError(t, err)

	// An invitation of another school for the same address stays untouched.
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	otherRole := testpkg.CreateTestRoleForTenant(t, db, "invited-elsewhere", otherSchool)
	elsewhere, err := env.service.CreateInvitation(testpkg.TenantContext(otherSchool), authService.InvitationRequest{
		Email: address, RoleID: otherRole.ID, TenantID: otherSchool, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	_, err = env.service.CreateInvitation(ctx, request)
	require.NoError(t, err)
	assert.False(t, env.storedInvitation(t, elsewhere.ID).IsUsed(), "another school's invitation is not spent")
}

// The inviting account may not hand out a role beyond its own authority, and
// a Lehrkraft is never combined with the caregiver upgrade (#1772).
func TestCreateInvitationRefusesRolesTheInviterMayNotGrant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-escalation-creator")
	adminRole, err := authService.ResolveSystemRoleByName(ctx, env.repos.Role, "admin")
	require.NoError(t, err)
	require.NotNil(t, adminRole, "the seeded admin role must exist")
	lehrkraft, err := authService.ResolveSystemRoleByName(ctx, env.repos.Role, "lehrkraft")
	require.NoError(t, err)
	require.NotNil(t, lehrkraft)

	_, err = env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("escalation"), RoleID: adminRole.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersCreatePermission},
	})
	require.ErrorIs(t, err, authService.ErrRoleGrantNotPermitted,
		"users:create alone must not hand out an admin-tier role")

	_, err = env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("lehrkraft-caregiver"), RoleID: lehrkraft.ID, CreatedBy: creator.ID,
		CaregiverEnabled: true, FirstName: testpkg.StrPtr("Lena"), LastName: testpkg.StrPtr("Lehrkraft"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.ErrorIs(t, err, authService.ErrLehrkraftNoCaregiver)

	// An operator-issued invitation carries no tenant permissions and is
	// still allowed to hand out the role.
	operatorInvitation, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("operator-invited"), RoleID: adminRole.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"), OperatorGrant: true,
	})
	require.NoError(t, err)
	assert.NotNil(t, operatorInvitation)
}

// A Lehrkraft accepts on the school portal, because that is where the login
// of that role lives (#2207).
func TestValidateInvitationNamesTheAcceptancePortal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-portal-creator")
	lehrkraft, err := authService.ResolveSystemRoleByName(ctx, env.repos.Role, "lehrkraft")
	require.NoError(t, err)

	invitation, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("school-portal"), RoleID: lehrkraft.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Lena"), LastName: testpkg.StrPtr("Lehrkraft"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	preview, err := env.service.ValidateInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, "school", preview.TargetPortal)
}

// An invitation to a school the account can already sign in to is refused,
// and an address with an account elsewhere keeps its credentials.
func TestCreateInvitationRefusesAnAccountThatAlreadyHasAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-member-creator")
	role := testpkg.CreateTestRole(t, db, "invited-member")
	member := testpkg.CreateTestAccount(t, db, "invite-existing-member")
	testpkg.EnsureAccountTenant(t, db, member.ID, testpkg.Tenant(t))

	_, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: member.Email, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.ErrorIs(t, err, authService.ErrAccountAlreadyHasTenantAccess)

	// The same address is invitable at another school.
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	otherRole := testpkg.CreateTestRoleForTenant(t, db, "invited-member-elsewhere", otherSchool)
	invitation, err := env.service.CreateInvitation(testpkg.TenantContext(otherSchool), authService.InvitationRequest{
		Email: member.Email, RoleID: otherRole.ID, TenantID: otherSchool, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	preview, err := env.service.ValidateInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.True(t, preview.RequiresAccountLogin, "an existing account signs in before accepting")
	_, err = env.service.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, authService.ErrInvitationOwnerRequired,
		"an invitation never takes over an existing account")
	assert.False(t, env.storedInvitation(t, invitation.ID).IsUsed(), "the refusal leaves the invitation redeemable")
}

// Accepting needs a name: neither the registration nor the invitation may
// leave it empty, and the two password fields must match.
func TestAcceptInvitationRefusesIncompleteRegistrations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-incomplete-creator")
	role := testpkg.CreateTestRole(t, db, "invited-incomplete")
	invitation, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("incomplete"), RoleID: role.ID, CreatedBy: creator.ID,
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	_, err = env.service.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		FirstName: "Ada", LastName: "Lovelace", Password: invitationPassword, ConfirmPassword: "different",
	})
	require.ErrorIs(t, err, authService.ErrPasswordMismatch)

	_, err = env.service.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		FirstName: "Ada", LastName: "Lovelace", Password: "weak", ConfirmPassword: "weak",
	})
	require.ErrorIs(t, err, authService.ErrPasswordTooWeak)

	_, err = env.service.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, authService.ErrInvitationNameRequired)
	assert.False(t, env.storedInvitation(t, invitation.ID).IsUsed(), "a refused acceptance leaves the invitation redeemable")
}

// Resending extends the link and starts its delivery bookkeeping over; a
// spent or expired invitation is never revived.
func TestResendInvitationExtendsOnlyRedeemableInvitations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-resend-creator")
	role := testpkg.CreateTestRole(t, db, "invited-resend")
	invitation, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("resend"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET expires_at = NOW() + INTERVAL '1 minute', email_error = 'smtp down', email_sent_at = NOW() WHERE id = ?`, invitation.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, env.service.ResendInvitation(ctx, invitation.ID, creator.ID))
	resent := env.storedInvitation(t, invitation.ID)
	assert.True(t, resent.ExpiresAt.After(time.Now().Add(time.Hour)), "the link is extended")

	require.NoError(t, env.service.RevokeInvitation(ctx, invitation.ID, creator.ID))
	require.ErrorIs(t, env.service.ResendInvitation(ctx, invitation.ID, creator.ID), authService.ErrInvitationUsed,
		"a spent invitation is not resent")
	require.ErrorIs(t, env.service.RevokeInvitation(ctx, invitation.ID, creator.ID), authService.ErrInvitationUsed)
	require.ErrorIs(t, env.service.ResendInvitation(ctx, invitation.ID+1_000_000, creator.ID), authService.ErrInvitationNotFound)

	expired, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("resend-expired"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = ?`, expired.ID).Exec(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, env.service.ResendInvitation(ctx, expired.ID, creator.ID), authService.ErrInvitationExpired,
		"an expired invitation is never revived")
}

// The pending listing, the school-wide invalidation and the cleanup keep the
// invitations that are still usable.
func TestInvitationListingInvalidationAndCleanup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-listing-creator")
	role := testpkg.CreateTestRole(t, db, "invited-listing")
	request := func(prefix string) authService.InvitationRequest {
		return authService.InvitationRequest{
			Email: inviteeAddress(prefix), RoleID: role.ID, CreatedBy: creator.ID,
			FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
			ActorPermissions: []string{usersManagePermission},
		}
	}
	pending, err := env.service.CreateInvitation(ctx, request("listing-pending"))
	require.NoError(t, err)
	expired, err := env.service.CreateInvitation(ctx, request("listing-expired"))
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = ?`, expired.ID).Exec(ctx)
	require.NoError(t, err)
	spent, err := env.service.CreateInvitation(ctx, request("listing-spent"))
	require.NoError(t, err)
	require.NoError(t, env.service.RevokeInvitation(ctx, spent.ID, creator.ID))

	listed, err := env.service.ListPendingInvitations(ctx)
	require.NoError(t, err)
	ids := make([]int64, 0, len(listed))
	for _, entry := range listed {
		ids = append(ids, entry.ID)
	}
	assert.Contains(t, ids, pending.ID)
	assert.NotContains(t, ids, expired.ID, "an expired invitation is not pending")
	assert.NotContains(t, ids, spent.ID, "a spent invitation is not pending")

	invalidated, err := env.service.InvalidatePendingInvitationsByTenantID(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.Equal(t, 2, invalidated, "the pending and the expired invitation of the school are spent")
	assert.True(t, env.storedInvitation(t, pending.ID).IsUsed())

	deleted, err := env.service.CleanupExpiredInvitations(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, deleted, "only the expired invitation is deleted")
	assert.True(t, env.storedInvitation(t, pending.ID).IsUsed(), "a spent but unexpired invitation is kept")
}

// The accept response carries the school's subdomain, which tenant routing
// resolves hosts by (#1977).
func TestInvitationSubdomainFollowsTheSchoolHost(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-subdomain-creator")
	role := testpkg.CreateTestRole(t, db, "invited-subdomain")
	subdomain := fmt.Sprintf("schule-%d", time.Now().UnixNano())
	_, err := db.NewRaw(`UPDATE platform.schools SET subdomain = ? WHERE id = ?`, subdomain, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	invitation, err := env.service.CreateInvitation(ctx, authService.InvitationRequest{
		Email: inviteeAddress("subdomain"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	assert.Equal(t, subdomain, env.service.GetTenantSubdomainForToken(context.Background(), invitation.Token))
	assert.Empty(t, env.service.GetTenantSubdomainForToken(context.Background(), "unknown-token"),
		"an unknown token answers without a host")
}

// An acceptance writes the account, its school mapping, the role and the
// identity chain in one transaction: a failure inside the chain rolls every
// earlier write back, and a retry succeeds from the untouched state.
func TestAcceptInvitationRollsBackEveryWriteOfTheFailedChain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	provisioningErr := errors.New("staff insert failed")
	failing, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db), services.WithAuthTestStaffCreateFailure(provisioningErr))
	require.NoError(t, err)
	working, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	env := newInvitationEnv(t, db)
	ctx := testpkg.Ctx(t)
	creator := testpkg.CreateTestAccount(t, db, "invite-rollback-creator")
	// The Betreuer tier needs the staff record the failing directory refuses.
	role, err := authService.ResolveSystemRoleByName(ctx, newInvitationEnv(t, db).repos.Role, "user")
	require.NoError(t, err)
	require.NotNil(t, role)
	address := inviteeAddress("rollback")

	invitation, err := failing.Invitation.CreateInvitation(ctx, authService.InvitationRequest{
		Email: address, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	_, err = failing.Invitation.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, provisioningErr)

	_, err = env.repos.Account.FindByEmail(context.Background(), address)
	require.Error(t, err, "the account must be rolled back")
	assert.False(t, env.storedInvitation(t, invitation.ID).IsUsed(), "the invitation stays redeemable for the retry")

	account, err := working.Invitation.AcceptInvitation(context.Background(), invitation.Token, authService.UserRegistrationData{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.NoError(t, err, "the retry starts from a clean state")
	testpkg.OwnTestAccount(t, db, account.ID)
	assert.True(t, env.storedInvitation(t, invitation.ID).IsUsed())
}
