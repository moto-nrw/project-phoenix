package behavior_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/moto-nrw/project-phoenix/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// loginGateScenario wires the composed Identity & Access module + real MFA service on top
// of the test DB so the LoginWithMFAGate branches can be exercised
// end-to-end. The MFA-required state is driven via the auth.mfa_overrides
// row (platform-wide) which short-circuits IsRequired without depending
// on any tenant settings registry. Post-#1430-round-2 the override no
// longer lives on the accounts row.
type loginGateScenario struct {
	t         *testing.T
	db        *bun.DB
	svc       *identityaccess.Module
	mfa       identityaccess.AccountMFA
	accountID int64
	email     string
	tenantID  int64
}

// loginGatePassword is generated at init time so it never appears as a
// string literal in the source tree. The fixed "Aa1!" prefix guarantees the
// strength rules (upper + lower + digit + special) are satisfied regardless
// of the hex-randomness that follows.
var loginGatePassword = randomTestPassword()

func randomTestPassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return "Aa1!" + hex.EncodeToString(b)
}

// newLoginGateScenario composes the auth service and the second factor over
// the test database, the way the service root does. The gate is always
// composed since #3331 — the school's security.mfa_mode decides, and these
// tests drive it through the platform-wide override row instead, which
// short-circuits the verdict without a settings registry.
func newLoginGateScenario(t *testing.T, options ...services.AuthTestOption) *loginGateScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	module := newMFATestModule(t, db, withDeliveringMailer(options)...)
	svc := module.Auth
	require.NotNil(t, svc, "the composed module serves the login gate")

	emailAddr := uniqueLoginEmail(t)
	acc := testpkg.CreateTestAccountWithPassword(t, db, emailAddr, loginGatePassword)

	tenantID := testpkg.UniqueTestTenantID(t)
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
		t: t, db: db, svc: svc, mfa: module.MFA,
		accountID: acc.ID, email: emailAddr, tenantID: tenantID,
	}
}

// setOverride writes a platform-wide row in auth.mfa_overrides so we
// can flip IsRequired regardless of tenant settings. Goes through raw
// SQL to keep the helper independent of the MFAService surface (which
// is the SUT in these tests).
func (sc *loginGateScenario) setOverride(override string) {
	sc.t.Helper()
	if override == identityaccess.MFAAdminOverrideNone {
		_, err := sc.db.NewDelete().
			Table("auth.mfa_overrides").
			Where("account_id = ? AND tenant_id IS NULL", sc.accountID).
			Exec(context.Background())
		require.NoError(sc.t, err)
		return
	}
	_, err := sc.db.ExecContext(context.Background(), `
		INSERT INTO auth.mfa_overrides
			(account_id, tenant_id, override, set_by, set_by_type, reason)
		VALUES (?, NULL, ?, 0, 'operator', 'test fixture')
		ON CONFLICT (account_id) WHERE tenant_id IS NULL
		DO UPDATE SET override = EXCLUDED.override, updated_at = CURRENT_TIMESTAMP`,
		sc.accountID, override)
	require.NoError(sc.t, err)
}

// A composed gate whose school leaves security.mfa_mode off issues the
// token pair straight away, with no challenge and no enrollment redirect.
func TestLoginWithMFAGate_GateOff_ReturnsTokens(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t)

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)
	assert.NotEmpty(t, result.AccessToken, "access token must be issued when the MFA gate is off")
	assert.NotEmpty(t, result.RefreshToken, "refresh token must be issued")
	assert.Empty(t, result.ChallengeToken, "no challenge token while the gate is off")
	assert.False(t, result.MFAEnrollmentRequired, "no enrollment flag when MFA disabled")
}

func TestLoginWithMFAGate_MFANotRequired_ReturnsTokens(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t)
	sc.setOverride(identityaccess.MFAAdminOverrideForceOff) // explicit "MFA off for this account"

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, identityaccess.LoginStatusAuthenticated, result.Status)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.False(t, result.MFAEnrollmentRequired,
		"force_off must not trigger the enrollment redirect")
}

func TestLoginWithMFAGate_MFARequiredNotEnrolled_IssuesEnrollmentToken(t *testing.T) {
	t.Parallel()

	// Post-#1430 review (Item #1) contract: unenrolled accounts on an
	// mfa-required tenant get an ENROLLMENT-SCOPED JWT in AccessToken
	// (not a full session) and NO refresh token. The previous design
	// returned `Status: authenticated` plus a full token pair, which
	// allowed bypassing MFA entirely by skipping enrollment.
	sc := newLoginGateScenario(t)
	sc.setOverride(identityaccess.MFAAdminOverrideForceOn) // require MFA, but no credential row exists

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, identityaccess.LoginStatusMFAEnrollmentRequired, result.Status,
		"unenrolled accounts must receive the narrow enrollment status, not a full session")
	assert.NotEmpty(t, result.AccessToken,
		"AccessToken carries the enrollment-scoped JWT so the frontend can call /mfa/enroll/*")
	assert.Empty(t, result.RefreshToken,
		"no refresh token before MFA enrollment — closes the pre-MFA-bypass hole")
	assert.True(t, result.MFAEnrollmentRequired,
		"MFAEnrollmentRequired is retained for legacy clients that branch on the boolean")
	assert.Empty(t, result.ChallengeToken, "no challenge issued before enrollment")
	assert.NotEmpty(t, result.MaskedEmail, "the enrollment screen renders the masked address")
}

func TestLoginWithMFAGate_MFARequiredAndEnrolled_ReturnsChallenge(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t)
	sc.setOverride(identityaccess.MFAAdminOverrideForceOn)
	require.NoError(t, sc.mfa.EnrollMFA(context.Background(), sc.accountID),
		"enrolment row must exist for the MFA-required-and-enrolled branch")

	result, err := sc.svc.LoginWithMFAGate(
		context.Background(), sc.email, loginGatePassword, "127.0.0.1", "ua-test", "", "",
	)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, identityaccess.LoginStatusMFARequired, result.Status,
		"enrolled accounts must be gated behind the second-factor challenge")
	assert.NotEmpty(t, result.ChallengeToken)
	assert.NotEmpty(t, result.MaskedEmail, "frontend renders the masked address on the challenge screen")
	assert.Empty(t, result.AccessToken, "no access token before challenge verification")
	assert.Empty(t, result.RefreshToken, "no refresh token before challenge verification")
}

func TestLoginWithMFAGate_WrongPassword_ReturnsAuthError(t *testing.T) {
	t.Parallel()

	sc := newLoginGateScenario(t)

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
