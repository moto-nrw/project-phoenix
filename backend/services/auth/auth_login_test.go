package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// loginGateScenario wires the real auth.Service + real MFA service on top
// of the test DB so the LoginWithMFAGate branches can be exercised
// end-to-end. The MFA-required state is driven via mfa_admin_override on
// the account, which short-circuits IsRequired without depending on any
// tenant settings registry.
type loginGateScenario struct {
	t         *testing.T
	db        *bun.DB
	svc       *auth.Service
	mfa       auth.MFAService
	accountID int64
	email     string
	tenantID  int64
}

const (
	loginGatePassword  = "C0rrectHorse!Test"
	loginGateJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"
)

func newLoginGateScenario(t *testing.T, withMFA bool) *loginGateScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)

	authCfg, err := auth.NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	svc, err := auth.NewService(repos, authCfg, db, nil)
	require.NoError(t, err)

	var mfaSvc auth.MFAService
	if withMFA {
		tokenAuth, err := authjwt.NewTokenAuthWithSecret(loginGateJWTSecret)
		require.NoError(t, err)
		mfaSvc, err = auth.NewMFAService(auth.MFAServiceConfig{
			Repos:     repos,
			TokenAuth: tokenAuth,
			JWTSecret: loginGateJWTSecret,
			DB:        db,
		})
		require.NoError(t, err)
		svc.SetMFAService(mfaSvc)
	}

	emailAddr := uniqueLoginEmail(t)
	acc := testpkg.CreateTestAccountWithPassword(t, db, emailAddr, loginGatePassword)

	const tenantID = int64(70010001)
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantID)

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.NewDelete().Table("auth.mfa_credentials").Where("account_id = ?", acc.ID).Exec(ctx)
		_, _ = db.NewDelete().Table("auth.mfa_email_challenges").Where("account_id = ?", acc.ID).Exec(ctx)
		_, _ = db.NewDelete().Table("auth.account_tenants").Where("account_id = ?", acc.ID).Exec(ctx)
		_, _ = db.NewDelete().Table("auth.accounts").Where("id = ?", acc.ID).Exec(ctx)
	})

	return &loginGateScenario{
		t: t, db: db, svc: svc, mfa: mfaSvc,
		accountID: acc.ID, email: emailAddr, tenantID: tenantID,
	}
}

// setOverride writes mfa_admin_override directly so we can flip
// IsRequired regardless of tenant settings.
func (sc *loginGateScenario) setOverride(override string) {
	sc.t.Helper()
	_, err := sc.db.NewUpdate().
		Table("auth.accounts").
		Set("mfa_admin_override = ?", override).
		Where("id = ?", sc.accountID).
		Exec(context.Background())
	require.NoError(sc.t, err)
}

func TestLoginWithMFAGate_NoMFAService_ReturnsTokens(t *testing.T) {
	sc := newLoginGateScenario(t, false)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, auth.LoginStatusAuthenticated, result.Status)
	assert.NotEmpty(t, result.AccessToken, "access token must be issued when MFA gate is off")
	assert.NotEmpty(t, result.RefreshToken, "refresh token must be issued")
	assert.Empty(t, result.ChallengeToken, "no challenge token without MFA gate")
	assert.False(t, result.MFAEnrollmentRequired, "no enrollment flag when MFA disabled")
}

func TestLoginWithMFAGate_MFANotRequired_ReturnsTokens(t *testing.T) {
	sc := newLoginGateScenario(t, true)
	sc.setOverride(auth.MFAAdminOverrideForceOff) // explicit "MFA off for this account"

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, auth.LoginStatusAuthenticated, result.Status)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.False(t, result.MFAEnrollmentRequired,
		"force_off must not trigger the enrollment redirect")
}

func TestLoginWithMFAGate_MFARequiredNotEnrolled_FlagsEnrollment(t *testing.T) {
	sc := newLoginGateScenario(t, true)
	sc.setOverride(auth.MFAAdminOverrideForceOn) // require MFA, but no credential row exists

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, auth.LoginStatusAuthenticated, result.Status,
		"unenrolled accounts still get a token pair — frontend must redirect to enrollment")
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.True(t, result.MFAEnrollmentRequired,
		"MFAEnrollmentRequired must signal the forced-enrollment redirect")
	assert.Empty(t, result.ChallengeToken, "no challenge issued when account isn't enrolled yet")
}

func TestLoginWithMFAGate_MFARequiredAndEnrolled_ReturnsChallenge(t *testing.T) {
	sc := newLoginGateScenario(t, true)
	sc.setOverride(auth.MFAAdminOverrideForceOn)
	require.NoError(t, sc.mfa.Enroll(context.Background(), sc.accountID),
		"enrolment row must exist for the MFA-required-and-enrolled branch")

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, auth.LoginStatusMFARequired, result.Status,
		"enrolled accounts must be gated behind the second-factor challenge")
	assert.NotEmpty(t, result.ChallengeToken)
	assert.NotEmpty(t, result.MaskedEmail, "frontend renders the masked address on the challenge screen")
	assert.Empty(t, result.AccessToken, "no access token before challenge verification")
	assert.Empty(t, result.RefreshToken, "no refresh token before challenge verification")
}

func TestLoginWithMFAGate_WrongPassword_ReturnsAuthError(t *testing.T) {
	sc := newLoginGateScenario(t, false)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, "wrong-password", "127.0.0.1", "ua-test", "", "",
	)
	require.Error(t, err, "wrong password must not return a result")
	assert.Nil(t, result)
}

// --- helpers ---

func uniqueLoginEmail(t *testing.T) string {
	t.Helper()
	return "login-gate-" + safeForEmail(t.Name()) + "-" + time.Now().UTC().Format("20060102T150405.000000000") + "@test.local"
}

func safeForEmail(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		default:
			out = append(out, '-')
		}
	}
	return string(out)
}
