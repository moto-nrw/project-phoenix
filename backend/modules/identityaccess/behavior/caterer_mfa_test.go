package behavior_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The caterer's second factor (#4497, ADR 0077 point 7): voluntary, bound
// to the caterer's account and to no school, with the e-mail code, the
// remembered device and the operator's reset as on the other portals.

// catererMFAScenario is a caterer that supplies one school, and the module
// whose mailer the codes arrive at.
type catererMFAScenario struct {
	db       *bun.DB
	module   identityaccess.Owner
	caterers *identityaccess.CatererAccess
	mailer   *testpkg.CapturingMailer
	school   int64
	email    string
	account  identityaccess.CatererAcceptance
}

func newCatererMFAScenario(t *testing.T) catererMFAScenario {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	inviting := setupAuthFactory(t, db).Caterers()
	school := testpkg.Tenant(t)
	staff := testpkg.CreateTestAccount(t, db, "caterer-mfa-inviter")
	email := fmt.Sprintf("kueche-mfa-%d@example.com", time.Now().UnixNano())
	token := invite(t, db, inviting, school, staff.ID, email)
	acceptance, err := accept(t, db, inviting, school, token, identityaccess.CatererRegistration{Password: catererPassword, ConfirmPassword: catererPassword})
	require.NoError(t, err)
	forgetCaterer(t, db, acceptance)
	mailer := testpkg.NewCapturingMailer()
	module := newMFATestModule(t, db, api.WithAuthTestMailer(mailer), api.WithAuthTestMFABackoff(time.Millisecond))
	caterers := module.Auth.Caterers()
	require.NotNil(t, caterers)
	return catererMFAScenario{db: db, module: module.Auth, caterers: caterers, mailer: mailer, school: school, email: email, account: acceptance}
}

// lastCode is the code of the newest code mail to the caterer.
func (sc catererMFAScenario) lastCode(t *testing.T) string {
	t.Helper()
	messages := sc.mailer.Messages()
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Template != "mfa-email-code.html" || messages[i].To.Address != sc.email {
			continue
		}
		content, ok := messages[i].Content.(map[string]any)
		require.True(t, ok)
		code, ok := content["Code"].(string)
		require.True(t, ok)
		return code
	}
	t.Fatal("no code mail to the caterer")
	return ""
}

func (sc catererMFAScenario) login(t *testing.T, cookie string) (identityaccess.CatererLoginResult, error) {
	t.Helper()
	return sc.caterers.LoginCaterer(context.Background(), identityaccess.CatererLoginRequest{
		Email: sc.email, Password: catererPassword, IPAddress: "192.0.2.40", UserAgent: "test", TrustedDeviceCookie: cookie,
	})
}

// switchOn switches the caterer's second factor on and returns the session
// the confirmation minted.
func (sc catererMFAScenario) switchOn(t *testing.T) identityaccess.CatererSession {
	t.Helper()
	ctx := context.Background()
	challenge, err := sc.caterers.StartCatererMFA(ctx, sc.account.AccountID, net.ParseIP("192.0.2.40"))
	require.NoError(t, err)
	require.NotEmpty(t, challenge.ChallengeToken)
	session, err := sc.caterers.ConfirmCatererMFA(ctx, identityaccess.CatererMFAConfirmation{
		AccountID: sc.account.AccountID, ChallengeToken: challenge.ChallengeToken, Code: sc.lastCode(t), IPAddress: "192.0.2.40", UserAgent: "test",
	})
	require.NoError(t, err)
	return session
}

func TestCatererSecondFactorIsVoluntaryAndGatesTheSignIn(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sc := newCatererMFAScenario(t)

	before, err := sc.login(t, "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.CatererLoginAuthenticated, before.Status, "without the factor the password signs in")
	state, err := sc.caterers.CatererMFAStatus(ctx, sc.account.AccountID)
	require.NoError(t, err)
	assert.False(t, state.Enabled)

	proven := sc.switchOn(t)
	_, err = sc.module.ValidateSessionTokens(ctx, proven.AccessToken, proven.RefreshToken, "caterer")
	require.NoError(t, err, "switching the factor on hands over a caterer session")
	state, err = sc.caterers.CatererMFAStatus(ctx, sc.account.AccountID)
	require.NoError(t, err)
	assert.True(t, state.Enabled)

	_, _, err = sc.module.RefreshTokenWithAudit(ctx, before.RefreshToken, "", "", "")
	require.Error(t, err, "a session minted before the factor ends at its next refresh")
	_, renewed, err := sc.module.RefreshTokenWithAudit(ctx, proven.RefreshToken, "", "", "")
	require.NoError(t, err, "the proven session lives on")
	require.NotEmpty(t, renewed)

	challenged, err := sc.login(t, "")
	require.NoError(t, err)
	require.Equal(t, identityaccess.CatererLoginMFARequired, challenged.Status)
	assert.Empty(t, challenged.AccessToken, "no session before the code")
	assert.Positive(t, challenged.TrustedDeviceDays, "the registry default lets the caterer remember a device")

	_, err = sc.caterers.VerifyCatererMFA(ctx, identityaccess.CatererMFAVerification{ChallengeToken: challenged.ChallengeToken, Code: "000000"})
	require.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid)
	verified, err := sc.caterers.VerifyCatererMFA(ctx, identityaccess.CatererMFAVerification{
		ChallengeToken: challenged.ChallengeToken, Code: sc.lastCode(t), RememberDevice: true, IPAddress: "192.0.2.40", UserAgent: "test",
	})
	require.NoError(t, err)
	_, err = sc.module.ValidateSessionTokens(ctx, verified.AccessToken, verified.RefreshToken, "caterer")
	require.NoError(t, err)
	for _, other := range []string{"tenant", "parent", "school", "org"} {
		_, err = sc.module.ValidateSessionTokens(ctx, verified.AccessToken, verified.RefreshToken, other)
		require.Error(t, err, "the caterer session stays out of portal %s", other)
	}
	require.NotEmpty(t, verified.TrustedDeviceCookie, "the caterer asked to remember the device")

	remembered, err := sc.login(t, verified.TrustedDeviceCookie)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.CatererLoginAuthenticated, remembered.Status, "a remembered device skips the code")
	devices, err := sc.caterers.ListCatererTrustedDevices(ctx, sc.account.AccountID)
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Zero(t, devices[0].TenantID, "the caterer's device names no school")

	err = sc.caterers.DisableCatererMFA(ctx, sc.account.AccountID, "wrong-password", "192.0.2.40", "test")
	require.ErrorIs(t, err, identityaccess.ErrInvalidCredentials)
	require.NoError(t, sc.caterers.DisableCatererMFA(ctx, sc.account.AccountID, catererPassword, "192.0.2.40", "test"))
	after, err := sc.login(t, verified.TrustedDeviceCookie)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.CatererLoginAuthenticated, after.Status)
	devices, err = sc.caterers.ListCatererTrustedDevices(ctx, sc.account.AccountID)
	require.NoError(t, err)
	assert.Empty(t, devices, "switching off forgets the remembered devices")
}

// The MFA gate of the caterer runs on the caterer's own choice, never on a
// school's mode: a school that requires the factor of all its staff does not
// reach the caterer it is supplied by, and a caterer challenge is
// redeemable only on the caterer portal.
func TestCatererSecondFactorIgnoresTheSchoolAndStaysInItsPortal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sc := newCatererMFAScenario(t)
	writeTenantSettingOverride(t, sc.db, sc.school, settingKeyMFAMode, mfaModeRequiredAll)

	result, err := sc.login(t, "")
	require.NoError(t, err)
	require.Equal(t, identityaccess.CatererLoginAuthenticated, result.Status, "the school's mode does not reach the caterer")
	_, _, err = sc.module.RefreshTokenWithAudit(ctx, result.RefreshToken, "", "", "")
	require.NoError(t, err, "nor its refresh")

	sc.switchOn(t)
	challenged, err := sc.login(t, "")
	require.NoError(t, err)
	require.Equal(t, identityaccess.CatererLoginMFARequired, challenged.Status)
	_, err = sc.module.VerifyMFAChallengeForScope(ctx, challenged.ChallengeToken, sc.lastCode(t), identityaccess.MFAChallengeScopeTenant)
	require.ErrorIs(t, err, identityaccess.ErrMFAUnsupportedScope, "a caterer code opens no school")
}

func TestOperatorResetsTheCaterersSecondFactor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sc := newCatererMFAScenario(t)
	operator := testpkg.CreateTestOperator(t, sc.db)
	sc.switchOn(t)

	caterers, err := sc.caterers.ListCaterersForOperator(ctx)
	require.NoError(t, err)
	var listed *identityaccess.CatererSecuritySummary
	for i := range caterers {
		if caterers[i].AccountID == sc.account.AccountID {
			listed = &caterers[i]
		}
	}
	require.NotNil(t, listed, "the operator lists the caterer")
	assert.Equal(t, sc.account.CatererID, listed.ID)
	assert.Equal(t, sc.email, listed.Email)
	assert.True(t, listed.MFAEnabled)

	err = sc.caterers.OperatorResetCatererMFA(ctx, operator.ID, sc.account.CatererID, " x ")
	require.ErrorIs(t, err, identityaccess.ErrCatererMFAResetReasonRequired)
	err = sc.caterers.OperatorResetCatererMFA(ctx, operator.ID, -1, "Gerät verloren")
	require.ErrorIs(t, err, identityaccess.ErrCatererNotFound)

	require.NoError(t, sc.caterers.OperatorResetCatererMFA(ctx, operator.ID, sc.account.CatererID, "Gerät verloren"))
	state, err := sc.caterers.CatererMFAStatus(ctx, sc.account.AccountID)
	require.NoError(t, err)
	assert.False(t, state.Enabled, "the reset switches the factor off")
	result, err := sc.login(t, "")
	require.NoError(t, err)
	assert.Equal(t, identityaccess.CatererLoginAuthenticated, result.Status)

	row := waitForOperatorAuditLog(t, sc.db, operator.ID, sc.account.AccountID, operatorActionMFAAdminOverride, 3*time.Second)
	require.NotNil(t, row, "the reset is in the operator audit")
	changes := row.changes(t)
	assert.Equal(t, "disable", changes["action"])
	assert.Equal(t, "caterer", changes["scope"])
	assert.Equal(t, "Gerät verloren", changes["reason"])

	t.Cleanup(func() {
		_, err := sc.db.NewRaw(`DELETE FROM platform.email_outbox WHERE tenant_id IS NULL AND related_entity_type = 'caterer_mfa_reset' AND related_entity_id = ?`,
			sc.account.AccountID).Exec(context.Background())
		require.NoError(t, err)
	})
	var notices int
	require.NoError(t, sc.db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE related_entity_type = 'caterer_mfa_reset' AND related_entity_id = ?`,
		sc.account.AccountID).Scan(ctx, &notices))
	assert.Equal(t, 1, notices, "the caterer gets one notice of the reset, committed with it")
}
