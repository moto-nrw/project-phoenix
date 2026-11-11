// A parent without a school sets up an account through the link of a holiday
// registration and signs in to the parents portal at the Träger (ADR 0216
// points 5 to 7). The link is no session; the session names the Träger and
// no school, and it ends once the access or the Träger is gone.
package behavior_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var holidayParentPassword = testpkg.NewTestPassword()

// holidayParentModule is the composed owner with the setup link it serves
// beside its Owner facade.
type holidayParentModule struct{ identityaccess.Owner }

func (m holidayParentModule) setups() identityaccess.HolidayParentSetups {
	setups, _ := m.Owner.(identityaccess.HolidayParentSetups)
	return setups
}

func (m holidayParentModule) IssueHolidayParentSetupToken(email string, organizationID int64) (string, error) {
	return m.setups().IssueHolidayParentSetupToken(email, organizationID)
}

func (m holidayParentModule) PreviewHolidayParentSetup(ctx context.Context, token string) (identityaccess.HolidayParentSetupPreview, error) {
	return m.setups().PreviewHolidayParentSetup(ctx, token)
}

func (m holidayParentModule) CompleteHolidayParentSetup(ctx context.Context, token string, registration identityaccess.GuardianRegistration) (bool, error) {
	return m.setups().CompleteHolidayParentSetup(ctx, token, registration)
}

func newHolidayTraeger(t *testing.T, db *bun.DB, name string) int64 {
	t.Helper()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("ferien-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	})
	return organization.ID
}

// setUpHolidayParent redeems a fresh setup link for a new address and owns
// the account it creates.
func setUpHolidayParent(t *testing.T, db *bun.DB, module holidayParentModule, prefix string, organizationID int64) (string, int64) {
	t.Helper()
	email, _ := uniqueTestCredentials(prefix)
	token, err := module.IssueHolidayParentSetupToken(email, organizationID)
	require.NoError(t, err)
	existed, err := module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{Password: holidayParentPassword, ConfirmPassword: holidayParentPassword})
	require.NoError(t, err)
	require.False(t, existed)
	account, err := module.FindAccountByEmail(context.Background(), email)
	require.NoError(t, err)
	testpkg.OwnTestAccount(t, db, account.ID)
	return email, account.ID
}

func TestHolidayParentSetupCreatesAnAccountAndKeepsAnExistingPassword(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Einrichten")
	email, _ := uniqueTestCredentials("holiday-setup")

	token, err := module.IssueHolidayParentSetupToken("  "+email+" ", traeger)
	require.NoError(t, err)
	preview, err := module.PreviewHolidayParentSetup(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.HolidayParentSetupPreview{Email: email, TraegerName: "Träger Ferien Einrichten", AccountExists: false}, preview)

	weak, err := module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{Password: "kurz", ConfirmPassword: "kurz"})
	require.ErrorIs(t, err, identityaccess.ErrPasswordTooWeak)
	assert.False(t, weak)

	existed, err := module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{Password: holidayParentPassword, ConfirmPassword: holidayParentPassword})
	require.NoError(t, err)
	assert.False(t, existed, "the link created the account")
	account, err := module.FindAccountByEmail(context.Background(), email)
	require.NoError(t, err)
	testpkg.OwnTestAccount(t, db, account.ID)
	var schools, roles int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM auth.account_tenants WHERE account_id = ?),
		(SELECT count(*) FROM auth.account_roles WHERE account_id = ?)`, account.ID, account.ID).Scan(context.Background(), &schools, &roles))
	assert.Zero(t, schools, "the account has no school")
	assert.Zero(t, roles, "the account has no role")

	preview, err = module.PreviewHolidayParentSetup(context.Background(), token)
	require.NoError(t, err)
	assert.True(t, preview.AccountExists)
	other := testpkg.NewTestPassword()
	existed, err = module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{Password: other, ConfirmPassword: "does not matter"})
	require.NoError(t, err, "redeeming the link again changes nothing")
	assert.True(t, existed)
	var access int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.traeger_parent_access WHERE account_id = ? AND organization_id = ?`, account.ID, traeger).Scan(context.Background(), &access))
	assert.Equal(t, 1, access)

	_, _, err = module.LoginParentWithAudit(context.Background(), email, other, "", "")
	require.Error(t, err, "the second password was not written")
	_, _, err = module.LoginParentWithAudit(context.Background(), email, holidayParentPassword, "", "")
	require.NoError(t, err, "the account keeps its own password")
}

func TestHolidayParentSetupRefusesInvalidLinks(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Ungültig")
	email, accountID := setUpHolidayParent(t, db, module, "holiday-invalid", traeger)
	access, refresh, err := module.LoginParentWithAudit(context.Background(), email, holidayParentPassword, "", "")
	require.NoError(t, err)

	for name, token := range map[string]string{"garbage": "not-a-token", "access token": access, "refresh token": refresh} {
		_, err := module.PreviewHolidayParentSetup(context.Background(), token)
		assert.ErrorIs(t, err, identityaccess.ErrHolidayParentSetupInvalid, name)
		_, err = module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{Password: holidayParentPassword, ConfirmPassword: holidayParentPassword})
		assert.ErrorIs(t, err, identityaccess.ErrHolidayParentSetupInvalid, name)
	}

	setup, err := module.IssueHolidayParentSetupToken(email, traeger)
	require.NoError(t, err)
	_, _, err = module.RefreshTokenWithAudit(context.Background(), setup, "", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "a setup link is no refresh token")
	_, err = module.ValidateSessionTokens(context.Background(), setup, "", "parent")
	assert.ErrorIs(t, err, identityaccess.ErrInvalidToken, "a setup link is no access token")

	_, err = db.ExecContext(context.Background(), `UPDATE platform.organizations SET active = FALSE WHERE id = ?`, traeger)
	require.NoError(t, err)
	_, err = module.PreviewHolidayParentSetup(context.Background(), setup)
	assert.ErrorIs(t, err, identityaccess.ErrHolidayParentSetupInvalid, "an inactive Träger makes the link invalid")
	_, err = module.CompleteHolidayParentSetup(context.Background(), setup, identityaccess.GuardianRegistration{})
	assert.ErrorIs(t, err, identityaccess.ErrHolidayParentSetupInvalid)
	assert.NotZero(t, accountID)
}

func TestTraegerParentSessionNamesTheTraegerAndEndsWithItsAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Sitzung")
	email, accountID := setUpHolidayParent(t, db, module, "holiday-session", traeger)

	access, refresh, err := module.LoginParentWithAudit(context.Background(), email, holidayParentPassword, "", "")
	require.NoError(t, err, "the access at the Träger opens the parents portal")
	claims, err := module.ValidateSessionTokens(context.Background(), access, refresh, "parent")
	require.NoError(t, err)
	assert.Equal(t, "parent", claims.Scope)
	assert.Equal(t, traeger, claims.OrgID, "the session names the Träger")
	assert.Zero(t, claims.TenantID, "and no school")
	assert.Empty(t, claims.Permissions)
	var stored struct {
		TenantID       *int64 `bun:"tenant_id"`
		OrganizationID int64  `bun:"organization_id"`
	}
	require.NoError(t, db.NewRaw(`SELECT tenant_id, organization_id FROM auth.tokens WHERE account_id = ? AND portal_scope = 'parent'`, accountID).Scan(context.Background(), &stored))
	assert.Nil(t, stored.TenantID, "the session is kept at no school")
	assert.Equal(t, traeger, stored.OrganizationID)
	require.NoError(t, module.VerifyAccessSession(context.Background(), identityaccess.AccessSession{AccountID: accountID, Scope: claims.Scope, FamilyID: claims.FamilyID}))

	access, refresh, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	require.NoError(t, err, "the session renews while access and Träger hold")
	renewed, err := module.ValidateSessionTokens(context.Background(), access, "", "parent")
	require.NoError(t, err)
	assert.Equal(t, traeger, renewed.OrgID)

	_, err = db.ExecContext(context.Background(), `DELETE FROM auth.traeger_parent_access WHERE account_id = ?`, accountID)
	require.NoError(t, err)
	_, err = module.ValidateSessionTokens(context.Background(), access, "", "parent")
	assert.Equal(t, identityaccess.SessionEnded, identityaccess.SessionOutcomeOf(err), "validation ends the session without access: %v", err)
	_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrTenantNotFound, "refresh ends the session without access")
	_, _, err = module.LoginParentWithAudit(context.Background(), email, holidayParentPassword, "", "")
	assert.ErrorIs(t, err, identityaccess.ErrAccountNoGuardianRole, "without access the refusal of today stands")
}

func TestTraegerParentSessionEndsWithTheTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Ende")
	email, _ := setUpHolidayParent(t, db, module, "holiday-traeger-end", traeger)
	access, refresh, err := module.LoginParentWithAudit(context.Background(), email, holidayParentPassword, "", "")
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `UPDATE platform.organizations SET active = FALSE WHERE id = ?`, traeger)
	require.NoError(t, err)
	_, err = module.ValidateSessionTokens(context.Background(), access, refresh, "parent")
	assert.Equal(t, identityaccess.SessionEnded, identityaccess.SessionOutcomeOf(err), "an inactive Träger ends the session: %v", err)
	_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	assert.ErrorIs(t, err, identityaccess.ErrTenantNotFound)

	// Ending the Träger's sessions ends the parent sessions at it too.
	_, err = db.ExecContext(context.Background(), `UPDATE platform.organizations SET active = TRUE WHERE id = ?`, traeger)
	require.NoError(t, err)
	var ended int64
	err = testpkg.WithinAdminContext(t, context.Background(), db, func(ctx context.Context) error {
		var endErr error
		ended, endErr = module.EndTraegerSessions(ctx, traeger)
		return endErr
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, ended)
	_, _, err = module.RefreshTokenWithAudit(context.Background(), refresh, "", "", "")
	assert.Error(t, err)
}

// The sign-in opens the ordinary parent session for an account whose
// guardian school books Eltern-App, and otherwise the session at the Träger
// of its oldest access.
func TestParentLoginFallsBackToTheOldestTraegerAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	service := newFixtureAuthService(t, db, module.Owner)
	first := newHolidayTraeger(t, db, "Träger Ferien Erster")
	second := newHolidayTraeger(t, db, "Träger Ferien Zweiter")
	email, accountID := setUpHolidayParent(t, db, module, "holiday-order", second)
	token, err := module.IssueHolidayParentSetupToken(email, first)
	require.NoError(t, err)
	_, err = module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{})
	require.NoError(t, err)
	// The second access is the older one.
	_, err = db.ExecContext(context.Background(), `UPDATE auth.traeger_parent_access SET created_at = created_at - INTERVAL '1 day' WHERE account_id = ? AND organization_id = ?`, accountID, second)
	require.NoError(t, err)

	access, _, err := module.LoginParentWithAudit(context.Background(), email, holidayParentPassword, "", "")
	require.NoError(t, err)
	claims, err := module.ValidateSessionTokens(context.Background(), access, "", "parent")
	require.NoError(t, err)
	assert.Equal(t, second, claims.OrgID, "the oldest access opens the session")

	guardianEmail, guardianID := newGuardianAccount(t, db, service, "holiday-guardian", testpkg.Tenant(t))
	guardianToken, err := module.IssueHolidayParentSetupToken(guardianEmail, first)
	require.NoError(t, err)
	existed, err := module.CompleteHolidayParentSetup(context.Background(), guardianToken, identityaccess.GuardianRegistration{})
	require.NoError(t, err)
	assert.True(t, existed)
	access, _, err = module.LoginParentWithAudit(context.Background(), guardianEmail, testPassword, "", "")
	require.NoError(t, err)
	claims, err = module.ValidateSessionTokens(context.Background(), access, "", "parent")
	require.NoError(t, err)
	assert.Zero(t, claims.OrgID, "a guardian school that books Eltern-App opens the ordinary session")
	var pinned int64
	require.NoError(t, db.NewRaw(`SELECT tenant_id FROM auth.tokens WHERE account_id = ? AND portal_scope = 'parent'`, guardianID).Scan(context.Background(), &pinned))
	assert.Equal(t, testpkg.Tenant(t), pinned)
}

// The parents portal resets the password of an account set up at a Träger.
func TestPasswordResetSendsTheHolidayParentToTheParentsPortal(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Passwort")
	email, _ := setUpHolidayParent(t, db, module, "holiday-reset", traeger)
	forgetResetWindow(t, db, email)

	link, err := module.InitiatePasswordReset(context.Background(), email, identityaccess.PasswordResetScopeParent)
	require.NoError(t, err)
	require.NotNil(t, link, "an account with access at a Träger resets on the parents portal")
	// The reset mail of an account without a school is global identity
	// mail, which no tenant of the test owns.
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'password_reset' AND related_entity_id = ?`, link.ID).Exec(context.Background())
		require.NoError(t, err)
	})
}

// The access at a Träger never opens the parents portal past the booking
// lock (ADR 0216 point 7, #4895): a guardian whose school does not book
// Eltern-App keeps the refusal of that school although the account also
// holds access at a Träger.
func TestTraegerAccessDoesNotBypassTheParentsPortalBooking(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	service := newFixtureAuthService(t, db, module.Owner)
	traeger := newHolidayTraeger(t, db, "Träger Ferien Buchungssperre")
	tenantID := testpkg.Tenant(t)
	email, accountID := newGuardianAccount(t, db, service, "holiday-unbooked", tenantID)
	cancelModule(t, db, tenantID, moduleElternApp)
	token, err := module.IssueHolidayParentSetupToken(email, traeger)
	require.NoError(t, err)
	existed, err := module.CompleteHolidayParentSetup(context.Background(), token, identityaccess.GuardianRegistration{})
	require.NoError(t, err)
	require.True(t, existed)

	_, _, err = module.LoginParentWithAudit(context.Background(), email, testPassword, "", "")

	requirePortalNotBooked(t, err, "eltern_app")
	var sessions int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM auth.tokens WHERE account_id = ? AND portal_scope = 'parent'`, accountID).Scan(context.Background(), &sessions))
	assert.Zero(t, sessions, "no parent session at the Träger was opened")
}

// An account set up at a Träger counts as a parent account while its access
// and the Träger hold (ADR 0216): the acceptance mail links it to the parents
// portal instead of offering a new account.
func TestHasTraegerParentAccessFollowsTheAccessAndTheTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Konto da")
	_, accountID := setUpHolidayParent(t, db, module, "holiday-has-access", traeger)
	other := testpkg.CreateTestAccount(t, db, "holiday-no-access")

	held, err := module.setups().HasTraegerParentAccess(context.Background(), accountID)
	require.NoError(t, err)
	assert.True(t, held)
	held, err = module.setups().HasTraegerParentAccess(context.Background(), other.ID)
	require.NoError(t, err)
	assert.False(t, held, "an account without access holds none")

	_, err = db.ExecContext(context.Background(), `UPDATE platform.organizations SET active = FALSE WHERE id = ?`, traeger)
	require.NoError(t, err)
	held, err = module.setups().HasTraegerParentAccess(context.Background(), accountID)
	require.NoError(t, err)
	assert.False(t, held, "an inactive Träger opens nothing")
}
