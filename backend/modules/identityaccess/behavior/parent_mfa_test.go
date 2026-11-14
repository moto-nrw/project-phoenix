package behavior_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The parents' second factor (#7051): voluntary, the account's e-mail code,
// bound to the parents portal and to no school, with remembered devices of
// that portal only and the moto team's reset through a school of the
// account.

type parentMFAScenario struct {
	db      *bun.DB
	module  api.AuthTestModule
	parents *identityaccess.ParentAccess
	mailer  *testpkg.CapturingMailer
	school  int64
	email   string
	account int64
}

// parentPasswordLogin signs a parent in with the password and returns the
// session pair of an account without a second factor.
func parentPasswordLogin(ctx context.Context, owner identityaccess.Owner, email, password string) (string, string, error) {
	result, err := owner.(identityaccess.ParentPortalAccess).Parents().LoginParent(ctx, identityaccess.ParentLoginRequest{Email: email, Password: password})
	return result.AccessToken, result.RefreshToken, err
}

// parentsOf is the parents portal's sign-in the composed owner serves
// beside its Owner facade.
func parentsOf(t *testing.T, owner identityaccess.Owner) *identityaccess.ParentAccess {
	t.Helper()
	access, ok := owner.(identityaccess.ParentPortalAccess)
	require.True(t, ok, "the composed owner serves the parents portal")
	require.NotNil(t, access.Parents())
	return access.Parents()
}

func newParentMFAScenario(t *testing.T) parentMFAScenario {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	email, accountID := newGuardianAccount(t, db, setupAuthService(t, db), "parent-mfa", school)
	// A parents-portal device names no school, so no tenant owns it.
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.mfa_trusted_devices WHERE account_id = ?`, accountID).Exec(context.Background())
		require.NoError(t, err)
	})
	mailer := testpkg.NewCapturingMailer()
	module := newMFATestModule(t, db, api.WithAuthTestMailer(mailer), api.WithAuthTestMFABackoff(time.Millisecond))
	parents := parentsOf(t, module.Auth)
	return parentMFAScenario{db: db, module: module, parents: parents, mailer: mailer, school: school, email: email, account: accountID}
}

// lastMail is the newest mail of the template to the parent.
func (sc parentMFAScenario) lastMail(t *testing.T, template string) map[string]any {
	t.Helper()
	messages := sc.mailer.Messages()
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Template != template || messages[i].To.Address != sc.email {
			continue
		}
		content, ok := messages[i].Content.(map[string]any)
		require.True(t, ok)
		return content
	}
	t.Fatalf("no %s mail to the parent", template)
	return nil
}

func (sc parentMFAScenario) lastCode(t *testing.T) string {
	t.Helper()
	code, ok := sc.lastMail(t, "parent-mfa-email-code.html")["Code"].(string)
	require.True(t, ok)
	return code
}

func (sc parentMFAScenario) login(t *testing.T, cookie string) identityaccess.ParentLoginResult {
	t.Helper()
	result, err := sc.parents.LoginParent(context.Background(), identityaccess.ParentLoginRequest{
		Email: sc.email, Password: testPassword, IPAddress: "192.0.2.41", UserAgent: "test", TrustedDeviceCookie: cookie,
	})
	require.NoError(t, err)
	return result
}

func (sc parentMFAScenario) switchOn(t *testing.T) identityaccess.ParentSession {
	t.Helper()
	ctx := context.Background()
	challenge, err := sc.parents.StartParentMFA(ctx, sc.account, net.ParseIP("192.0.2.41"))
	require.NoError(t, err)
	session, err := sc.parents.ConfirmParentMFA(ctx, identityaccess.ParentMFAConfirmation{
		AccountID: sc.account, ChallengeToken: challenge.ChallengeToken, Code: sc.lastCode(t), IPAddress: "192.0.2.41", UserAgent: "test",
	})
	require.NoError(t, err)
	return session
}

func TestParentSecondFactorIsVoluntaryAndGatesTheSignIn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sc := newParentMFAScenario(t)
	auth := sc.module.Auth

	before := sc.login(t, "")
	require.Equal(t, identityaccess.LoginStatusAuthenticated, before.Status, "without the factor the password signs in")
	state, err := sc.parents.ParentMFAStatus(ctx, sc.account)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.ParentMFAState{Enabled: false, Available: true, MaskedEmail: state.MaskedEmail}, state)
	assert.NotEmpty(t, state.MaskedEmail)

	proven := sc.switchOn(t)
	_, err = auth.ValidateSessionTokens(ctx, proven.AccessToken, proven.RefreshToken, "parent")
	require.NoError(t, err, "switching the factor on hands over a parent session")
	_, _, err = auth.RefreshTokenWithAudit(ctx, before.RefreshToken, "", "", "")
	require.Error(t, err, "a session minted before the factor ends at its next refresh")
	_, _, err = auth.RefreshTokenWithAudit(ctx, proven.RefreshToken, "", "", "")
	require.NoError(t, err, "the proven session lives on")

	challenged := sc.login(t, "")
	require.Equal(t, identityaccess.LoginStatusMFARequired, challenged.Status)
	assert.Empty(t, challenged.AccessToken, "no session before the code")
	assert.Positive(t, challenged.TrustedDeviceDays, "the registry default lets the parent remember a device")
	_, err = auth.VerifyMFAChallengeForScope(ctx, challenged.ChallengeToken, sc.lastCode(t), identityaccess.MFAChallengeScopeTenant)
	require.ErrorIs(t, err, identityaccess.ErrMFAUnsupportedScope, "a parent code opens no school")

	challenged = sc.login(t, "")
	verified, err := sc.parents.VerifyParentMFA(ctx, identityaccess.ParentMFAVerification{
		ChallengeToken: challenged.ChallengeToken, Code: sc.lastCode(t), RememberDevice: true, IPAddress: "192.0.2.41", UserAgent: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, sc.email, verified.Email)
	require.NotEmpty(t, verified.TrustedDeviceCookie)
	for _, other := range []string{"tenant", "caterer", "school", "org"} {
		_, err = auth.ValidateSessionTokens(ctx, verified.AccessToken, verified.RefreshToken, other)
		require.Error(t, err, "the parent session stays out of portal %s", other)
	}
	notice := sc.lastMail(t, "parent-trusted-device-added.html")
	assert.Equal(t, "Neues gemerktes Gerät", notice["Title"], "the notice speaks the parents portal's language")

	remembered := sc.login(t, verified.TrustedDeviceCookie)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, remembered.Status, "a remembered device skips the code")
	devices, err := sc.parents.ListParentTrustedDevices(ctx, sc.account)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.True(t, devices[0].ParentPortal)
	assert.Zero(t, devices[0].TenantID, "the parent's device names no school")
	assert.False(t, auth.Caterers() != nil && hasCatererDevice(ctx, t, auth.Caterers(), sc.account), "the caterer portal does not see it")

	require.ErrorIs(t, sc.parents.DisableParentMFA(ctx, sc.account, "wrong-password", "", ""), identityaccess.ErrInvalidCredentials)
	require.NoError(t, sc.parents.DisableParentMFA(ctx, sc.account, testPassword, "", ""))
	after := sc.login(t, verified.TrustedDeviceCookie)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, after.Status)
	devices, err = sc.parents.ListParentTrustedDevices(ctx, sc.account)
	require.NoError(t, err)
	assert.Empty(t, devices, "switching off forgets the remembered devices")
	require.ErrorIs(t, sc.parents.DisableParentMFA(ctx, sc.account, testPassword, "", ""), identityaccess.ErrParentMFANotEnabled)
}

// hasCatererDevice reports whether the caterer portal lists a device of the
// account; an account that is no caterer's lists none.
func hasCatererDevice(ctx context.Context, t *testing.T, caterers *identityaccess.CatererAccess, accountID int64) bool {
	t.Helper()
	devices, err := caterers.ListCatererTrustedDevices(ctx, accountID)
	return err == nil && len(devices) > 0
}

// The parent's code mail is written in the language the parent chose for
// the portal.
func TestParentCodeMailSpeaksThePortalLanguage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sc := newParentMFAScenario(t)
	_, err := sc.db.NewRaw(`UPDATE auth.accounts SET portal_locale = 'en' WHERE id = ?`, sc.account).Exec(ctx)
	require.NoError(t, err)

	_, err = sc.parents.StartParentMFA(ctx, sc.account, nil)
	require.NoError(t, err)
	mail := sc.lastMail(t, "parent-mfa-email-code.html")
	assert.Equal(t, "Your sign-in code", mail["Title"])
	assert.Equal(t, "en", mail["Lang"])
}

// An account that has the e-mail factor from its work at a school is asked
// for it at the parents portal too: the factor belongs to the account.
func TestParentSignInAsksTheFactorTheAccountHasFromItsSchool(t *testing.T) {
	t.Parallel()
	sc := newParentMFAScenario(t)
	require.NoError(t, enrollMFA(context.Background(), t, sc.db, sc.module.MFA, sc.account))

	result := sc.login(t, "")
	assert.Equal(t, identityaccess.LoginStatusMFARequired, result.Status)
}

// The moto team resets a parent who lost the mailbox through the operator's
// reset at a school of the account; the password signs in again.
func TestOperatorResetsTheParentsSecondFactor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sc := newParentMFAScenario(t)
	operator := testpkg.CreateTestOperator(t, sc.db)
	sc.switchOn(t)
	require.Equal(t, identityaccess.LoginStatusMFARequired, sc.login(t, "").Status)

	t.Cleanup(func() {
		_, err := sc.db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'account_mfa_reset' AND related_entity_id = ?`,
			sc.account).Exec(context.Background())
		require.NoError(t, err)
	})
	require.NoError(t, sc.module.MFA.OperatorDisableMFA(ctx, operator.ID, sc.school, sc.account, "Postfach verloren"))
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, sc.login(t, "").Status)
	var notices int
	require.NoError(t, sc.db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE related_entity_type = 'account_mfa_reset' AND related_entity_id = ?`,
		sc.account).Scan(ctx, &notices))
	assert.Equal(t, 1, notices, "the parent learns of the reset")
}

// A parent at no school, set up for a holiday registration, signs in at the
// Träger without the factor and cannot switch it on: the moto team resets a
// lost factor through a school of the account, and there is none.
func TestTraegerParentCannotSwitchTheFactorOn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := testpkg.SetupTestDB(t)
	holiday := holidayParentModule{setupAuthFactory(t, db)}
	traeger := newHolidayTraeger(t, db, "Träger Ferien Zweiter Faktor")
	email, accountID := setUpHolidayParent(t, db, holiday, "holiday-mfa", traeger)
	parents := parentsOf(t, newMFATestModule(t, db).Auth)

	result, err := parents.LoginParent(ctx, identityaccess.ParentLoginRequest{Email: email, Password: holidayParentPassword})
	require.NoError(t, err)
	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)
	state, err := parents.ParentMFAStatus(ctx, accountID)
	require.NoError(t, err)
	assert.False(t, state.Available)
	_, err = parents.StartParentMFA(ctx, accountID, nil)
	require.ErrorIs(t, err, identityaccess.ErrParentMFANeedsSchool)
}
