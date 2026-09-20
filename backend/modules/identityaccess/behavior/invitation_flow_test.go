package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
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
	service services.InvitationCapability
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

	invitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
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

	preview, err := env.service.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.InvitationPortalTenant, preview.Portal)
	assert.False(t, preview.RequiresAccountLogin, "an unknown address signs up instead of signing in")
	assert.Equal(t, address, preview.Email)
	assert.Equal(t, role.Name, preview.RoleName)

	account, err := env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	testpkg.OwnTestAccount(t, db, account.ID)
	assert.Equal(t, address, account.Email)

	member, err := testpkg.ActiveAccountTenantExists(context.Background(), env.db, account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.True(t, member, "the invitee can sign in at the school")
	person, err := env.repos.Person.FindByAccountID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, person, "accepting provisions the identity the role needs")
	assert.Equal(t, "Ada", person.FirstName)
	assert.True(t, env.storedInvitation(t, invitation.ID).IsUsed())

	_, err = env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationUsed, "a spent invitation grants no second access")
	_, err = env.service.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.ErrorIs(t, err, identityaccess.ErrInvitationUsed)
	_, err = env.service.ValidateSchoolInvitation(context.Background(), "unknown-token")
	require.ErrorIs(t, err, identityaccess.ErrInvitationNotFound)
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
	request := identityaccess.SchoolInvitationRequest{
		Email: address, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	}

	first, err := env.service.CreateSchoolInvitation(ctx, request)
	require.NoError(t, err)
	request.Email = " " + strings.ToUpper(address) + " "
	second, err := env.service.CreateSchoolInvitation(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, address, second.Email)

	assert.True(t, env.storedInvitation(t, first.ID).IsUsed(), "the previous invitation is spent")
	_, err = env.service.ValidateSchoolInvitation(context.Background(), first.Token)
	require.ErrorIs(t, err, identityaccess.ErrInvitationUsed)
	_, err = env.service.ValidateSchoolInvitation(context.Background(), second.Token)
	require.NoError(t, err)

	// An invitation of another school for the same address stays untouched.
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	otherRole := testpkg.CreateTestRoleForTenant(t, db, "invited-elsewhere", otherSchool)
	elsewhere, err := env.service.CreateSchoolInvitation(testpkg.TenantContext(otherSchool), identityaccess.SchoolInvitationRequest{
		Email: address, RoleID: otherRole.ID, TenantID: otherSchool, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	_, err = env.service.CreateSchoolInvitation(ctx, request)
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
	adminRoleID := systemRoleID(t, db, "admin")
	lehrkraftID := systemRoleID(t, db, "lehrkraft")

	_, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("escalation"), RoleID: adminRoleID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersCreatePermission},
	})
	require.ErrorIs(t, err, identityaccess.ErrRoleGrantNotPermitted,
		"users:create alone must not hand out an admin-tier role")

	_, err = env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("lehrkraft-caregiver"), RoleID: lehrkraftID, CreatedBy: creator.ID,
		CaregiverEnabled: true, FirstName: testpkg.StrPtr("Lena"), LastName: testpkg.StrPtr("Lehrkraft"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.ErrorIs(t, err, identityaccess.ErrLehrkraftNoCaregiver)

	// An operator-issued invitation carries no tenant permissions and is
	// still allowed to hand out the role.
	operatorInvitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("operator-invited"), RoleID: adminRoleID, CreatedBy: creator.ID,
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
	lehrkraftID := systemRoleID(t, db, "lehrkraft")

	invitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("school-portal"), RoleID: lehrkraftID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Lena"), LastName: testpkg.StrPtr("Lehrkraft"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	preview, err := env.service.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.InvitationPortalSchool, preview.Portal)
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

	_, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: member.Email, RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.ErrorIs(t, err, identityaccess.ErrAccountAlreadyHasTenantAccess)

	// The same address is invitable at another school.
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	otherRole := testpkg.CreateTestRoleForTenant(t, db, "invited-member-elsewhere", otherSchool)
	invitation, err := env.service.CreateSchoolInvitation(testpkg.TenantContext(otherSchool), identityaccess.SchoolInvitationRequest{
		Email: member.Email, RoleID: otherRole.ID, TenantID: otherSchool, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	preview, err := env.service.ValidateSchoolInvitation(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.True(t, preview.RequiresAccountLogin, "an existing account signs in before accepting")
	_, err = env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationOwnerRequired,
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
	invitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("incomplete"), RoleID: role.ID, CreatedBy: creator.ID,
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	_, err = env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		FirstName: "Ada", LastName: "Lovelace", Password: invitationPassword, ConfirmPassword: "different",
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationPasswordMismatch)

	_, err = env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		FirstName: "Ada", LastName: "Lovelace", Password: "weak", ConfirmPassword: "weak",
	})
	require.ErrorIs(t, err, identityaccess.ErrPasswordTooWeak)

	_, err = env.service.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, identityaccess.ErrInvitationNameRequired)
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
	invitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("resend"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET expires_at = NOW() + INTERVAL '1 minute', email_error = 'smtp down', email_sent_at = NOW() WHERE id = ?`, invitation.ID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, env.service.ResendSchoolInvitation(ctx, invitation.ID, creator.ID))
	resent := env.storedInvitation(t, invitation.ID)
	assert.True(t, resent.ExpiresAt.After(time.Now().Add(time.Hour)), "the link is extended")

	require.NoError(t, env.service.RevokeSchoolInvitation(ctx, invitation.ID, creator.ID))
	require.ErrorIs(t, env.service.ResendSchoolInvitation(ctx, invitation.ID, creator.ID), identityaccess.ErrInvitationUsed,
		"a spent invitation is not resent")
	require.ErrorIs(t, env.service.RevokeSchoolInvitation(ctx, invitation.ID, creator.ID), identityaccess.ErrInvitationUsed)
	require.ErrorIs(t, env.service.ResendSchoolInvitation(ctx, invitation.ID+1_000_000, creator.ID), identityaccess.ErrInvitationNotFound)

	expired, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("resend-expired"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = ?`, expired.ID).Exec(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, env.service.ResendSchoolInvitation(ctx, expired.ID, creator.ID), identityaccess.ErrInvitationExpired,
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
	request := func(prefix string) identityaccess.SchoolInvitationRequest {
		return identityaccess.SchoolInvitationRequest{
			Email: inviteeAddress(prefix), RoleID: role.ID, CreatedBy: creator.ID,
			FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
			ActorPermissions: []string{usersManagePermission},
		}
	}
	pending, err := env.service.CreateSchoolInvitation(ctx, request("listing-pending"))
	require.NoError(t, err)
	expired, err := env.service.CreateSchoolInvitation(ctx, request("listing-expired"))
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.invitation_tokens SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = ?`, expired.ID).Exec(ctx)
	require.NoError(t, err)
	spent, err := env.service.CreateSchoolInvitation(ctx, request("listing-spent"))
	require.NoError(t, err)
	require.NoError(t, env.service.RevokeSchoolInvitation(ctx, spent.ID, creator.ID))

	listed, err := env.service.ListPendingSchoolInvitations(ctx)
	require.NoError(t, err)
	ids := make([]int64, 0, len(listed))
	var pendingEntry *identityaccess.SchoolInvitation
	for index, entry := range listed {
		ids = append(ids, entry.ID)
		if entry.ID == pending.ID {
			pendingEntry = &listed[index]
		}
	}
	assert.Contains(t, ids, pending.ID)
	assert.NotContains(t, ids, expired.ID, "an expired invitation is not pending")
	assert.NotContains(t, ids, spent.ID, "a spent invitation is not pending")
	require.NotNil(t, pendingEntry)
	assert.Equal(t, role.Name, pendingEntry.RoleName, "the pending list carries the invited role")
	assert.Equal(t, creator.Email, pendingEntry.CreatorEmail, "the pending list carries who sent the invitation")

	invalidated, err := env.service.RevokeTenantSchoolInvitations(ctx, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.Equal(t, 2, invalidated, "the pending and the expired invitation of the school are spent")
	assert.True(t, env.storedInvitation(t, pending.ID).IsUsed())

	deleted, err := env.service.DeleteExpiredSchoolInvitations(ctx)
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
	invitation, err := env.service.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("subdomain"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	assert.Equal(t, subdomain, env.service.SchoolInvitationSubdomain(context.Background(), invitation.Token))
	assert.Empty(t, env.service.SchoolInvitationSubdomain(context.Background(), "unknown-token"),
		"an unknown token answers without a host")
}

// Create queues the invitation mail after commit on a detached context.
// Reply-To still follows the invitation's school, not the stripped tenant (#1936).
func TestCreateInvitationMailUsesTheSchoolReplyTo(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	mailer := testpkg.NewCapturingMailer()
	module, err := services.NewAuthTestModule(db, testpkg.TenantRuntime(t, db),
		services.WithAuthTestMailer(mailer),
		services.WithAuthTestPasswordResetBackoff(time.Millisecond),
	)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	replyTo := fmt.Sprintf("ogs-%d@schule.test", time.Now().UnixNano())
	_, err = db.NewRaw(`UPDATE platform.schools SET email = ?, name = 'OGS Am Berg' WHERE id = ?`, replyTo, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	creator := testpkg.CreateTestAccount(t, db, "invite-replyto-creator")
	role := testpkg.CreateTestRole(t, db, "invited-replyto")

	_, err = module.Invitation.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: inviteeAddress("replyto"), RoleID: role.ID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)
	require.True(t, mailer.WaitForMessages(1, 2*time.Second), "create queues the invitation mail after commit")
	message := mailer.Messages()[0]
	assert.Equal(t, replyTo, message.ReplyTo.Address)
	assert.Equal(t, "OGS Am Berg", message.ReplyTo.Name)
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
	roleID := systemRoleID(t, db, "user")
	address := inviteeAddress("rollback")

	invitation, err := failing.Invitation.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: address, RoleID: roleID, CreatedBy: creator.ID,
		FirstName: testpkg.StrPtr("Ada"), LastName: testpkg.StrPtr("Lovelace"),
		ActorPermissions: []string{usersManagePermission},
	})
	require.NoError(t, err)

	_, err = failing.Invitation.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.ErrorIs(t, err, provisioningErr)

	_, err = env.repos.Account.FindByEmail(context.Background(), address)
	require.Error(t, err, "the account must be rolled back")
	assert.False(t, env.storedInvitation(t, invitation.ID).IsUsed(), "the invitation stays redeemable for the retry")

	account, err := working.Invitation.AcceptSchoolInvitation(context.Background(), invitation.Token, identityaccess.InvitationRegistration{
		Password: invitationPassword, ConfirmPassword: invitationPassword,
	})
	require.NoError(t, err, "the retry starts from a clean state")
	testpkg.OwnTestAccount(t, db, account.ID)
	assert.True(t, env.storedInvitation(t, invitation.ID).IsUsed())
}
