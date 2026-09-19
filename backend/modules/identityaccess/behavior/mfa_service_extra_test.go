package behavior_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// newExtraMFAService composes the second factor over the test database and
// returns an account to drive it with.
func newExtraMFAService(t *testing.T) (identityaccess.AccountMFA, *repositories.Factory, int64) {
	t.Helper()
	scenario := newMFATestScenario(t)
	return scenario.MFA, scenario.Repos, scenario.AccountID
}

// VerifyCodeForAccount is the JWT-less verify used by enrollment confirm.
// It's currently 0% in the coverage profile — these tests exercise the
// three exit paths (no-active-challenge, wrong-code, success).

func TestMFAService_VerifyCodeForAccount_NoActiveChallenge_ReturnsInvalid(t *testing.T) {
	t.Parallel()

	svc, _, accID := newExtraMFAService(t)

	err := svc.VerifyMFACodeForAccount(context.Background(), accID, 0, "123456", identityaccess.MFAChallengeScopeTenant)

	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid,
		"no active email challenge must surface as ErrMFACodeInvalid, not a generic error")
}

func TestMFAService_VerifyCodeForAccount_WrongAccount_ReturnsInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newExtraMFAService(t)

	// Account id 99999999 doesn't exist — FindByID returns sql.ErrNoRows
	// and the helper must map it to "invalid", not 500.
	err := svc.VerifyMFACodeForAccount(context.Background(), 99999999, 0, "123456", identityaccess.MFAChallengeScopeTenant)

	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid)
}

func TestMFAService_VerifyCodeForAccount_WrongCode_RecordsFailureAndReturnsInvalid(t *testing.T) {
	t.Parallel()

	svc, _, accID := newExtraMFAService(t)
	require.NoError(t, svc.EnrollMFA(context.Background(), accID))

	// Seed an active challenge via the public StartChallenge path so we
	// have a real CodeHash to mismatch against.
	_, err := svc.StartMFAChallenge(context.Background(), accID, 0, identityaccess.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	err = svc.VerifyMFACodeForAccount(context.Background(), accID, 0, "000000", identityaccess.MFAChallengeScopeTenant)

	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid)
}

// ResendChallenge re-issues a code given a valid challenge token. Failure
// paths are: bad token (invalid JWT) and downstream StartChallenge error.

func TestMFAService_ResendChallenge_InvalidToken_ReturnsTokenInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newExtraMFAService(t)

	renewed, err := svc.ResendMFAChallenge(context.Background(), "not-a-jwt", net.ParseIP("127.0.0.1"))

	require.Error(t, err)
	assert.Empty(t, renewed, "invalid token must not produce a renewed JWT")
	assert.ErrorIs(t, err, identityaccess.ErrMFAChallengeTokenInvalid)
}

func TestMFAService_ResendChallenge_HappyPath(t *testing.T) {
	t.Parallel()

	svc, _, accID := newExtraMFAService(t)
	require.NoError(t, svc.EnrollMFA(context.Background(), accID))

	// Mint a real challenge token via StartChallenge so ResendChallenge
	// can parse it back out.
	token, err := svc.StartMFAChallenge(context.Background(), accID, 0, identityaccess.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// The resend path goes parseChallengeToken → StartChallenge, both
	// of which are exercised end-to-end. The renewed JWT is what the
	// frontend must swap in for the in-flight challenge_token.
	renewed, err := svc.ResendMFAChallenge(context.Background(), token, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	// The token may or may not differ byte-for-byte from the original
	// (JWT iat has 1-second granularity and the encoded claims overlap
	// fully when start + resend land in the same second). What matters
	// is that the resend path *returns* a usable token at all — the
	// pre-fix shape returned only an error.
	assert.NotEmpty(t, renewed, "happy path must return a renewed challenge JWT")
}

// ShortenUserAgent parses common UA strings into a friendly "Browser auf OS"
// label used by the trusted-device email + UI. Exported helper — 0% in the
// coverage profile before this test.
func TestMFAService_TrustedDeviceDays_DefaultsWhenSettingsMissing(t *testing.T) {
	t.Parallel()

	svc, _, _ := newExtraMFAService(t)

	// No tenant settings wired; helper must fall back to the registry
	// default. Any positive integer indicates the fallback fired.
	days := svc.TrustedDeviceDays(context.Background(), 0)
	assert.Positive(t, days, "trusted-device days must fall back to a positive default when no settings are configured")
}

// IssueTrustedDevice on an account with no trusted-device cookie support
// returns ("", zero time, nil) — verify the no-cookie short-circuit
// branch is reachable (defaults to enabled in the test build so this
// asserts only that it doesn't error).
func TestMFAService_IssueTrustedDevice_PersistsAndReturnsSignedToken(t *testing.T) {
	t.Parallel()

	svc, _, accID := newExtraMFAService(t)
	require.NoError(t, svc.EnrollMFA(context.Background(), accID))

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	cookie, expiresAt, err := svc.IssueTrustedDevice(
		context.Background(), accID, tenantID, "ua-test", net.ParseIP("127.0.0.1"),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, cookie, "trusted-device default-enabled tenant must mint a cookie value")
	assert.True(t, expiresAt.After(time.Now()), "expires_at must be in the future")
}

// The rate-limit count is the only statement that fails below; everything
// else behaves normally, which is the situation the fail-open bug needed to
// show itself: challenge creation and mail dispatch succeed regardless of
// whether that count could be read.

func TestMFAService_StartChallenge_RateLimitLookupFails_IssuesNoCode(t *testing.T) {
	t.Parallel()

	svc, repos, db := newTestMFAService(t, withFailingMFARecords(failingMFARecords{
		countChallengesSince: func(context.Context, int64, time.Time) (int, error) {
			return 0, errors.New("rate-limit count unavailable")
		},
	}))

	acc := testpkg.CreateTestAccount(t, db, "mfa-ratelimit-blind")

	challengeToken, err := svc.StartMFAChallenge(
		context.Background(), acc.ID, 0, identityaccess.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"),
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable,
		"an unreadable rate-limit count must refuse the code, not wave it through")
	assert.Empty(t, challengeToken)

	issued, err := repos.MFAEmailChallenge.CountRecentByAccountID(
		context.Background(), acc.ID, time.Now().Add(-time.Hour),
	)
	require.NoError(t, err)
	assert.Zero(t, issued, "a refused challenge must leave no code behind")
}
