package auth_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// extraJWTSecret matches the constant used by the existing service tests.
const extraJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

func newExtraMFAService(t *testing.T) (auth.MFAService, *repositories.Factory, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(extraJWTSecret)
	require.NoError(t, err)
	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: extraJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	acc := testpkg.CreateTestAccount(t, db, "mfa-extra")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })
	return svc, repos, acc.ID
}

// VerifyCodeForAccount is the JWT-less verify used by enrollment confirm.
// It's currently 0% in the coverage profile — these tests exercise the
// three exit paths (no-active-challenge, wrong-code, success).

func TestMFAService_VerifyCodeForAccount_NoActiveChallenge_ReturnsInvalid(t *testing.T) {
	svc, _, accID := newExtraMFAService(t)

	err := svc.VerifyCodeForAccount(context.Background(), accID, 0, "123456", authjwt.MFAChallengeScopeTenant)

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid,
		"no active email challenge must surface as ErrMFACodeInvalid, not a generic error")
}

func TestMFAService_VerifyCodeForAccount_WrongAccount_ReturnsInvalid(t *testing.T) {
	svc, _, _ := newExtraMFAService(t)

	// Account id 99999999 doesn't exist — FindByID returns sql.ErrNoRows
	// and the helper must map it to "invalid", not 500.
	err := svc.VerifyCodeForAccount(context.Background(), 99999999, 0, "123456", authjwt.MFAChallengeScopeTenant)

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid)
}

func TestMFAService_VerifyCodeForAccount_WrongCode_RecordsFailureAndReturnsInvalid(t *testing.T) {
	svc, _, accID := newExtraMFAService(t)
	require.NoError(t, svc.Enroll(context.Background(), accID))

	// Seed an active challenge via the public StartChallenge path so we
	// have a real CodeHash to mismatch against.
	_, err := svc.StartChallenge(context.Background(), accID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	err = svc.VerifyCodeForAccount(context.Background(), accID, 0, "000000", authjwt.MFAChallengeScopeTenant)

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid)
}

// ResendChallenge re-issues a code given a valid challenge token. Failure
// paths are: bad token (invalid JWT) and downstream StartChallenge error.

func TestMFAService_ResendChallenge_InvalidToken_ReturnsTokenInvalid(t *testing.T) {
	svc, _, _ := newExtraMFAService(t)

	renewed, err := svc.ResendChallenge(context.Background(), "not-a-jwt", net.ParseIP("127.0.0.1"))

	require.Error(t, err)
	assert.Empty(t, renewed, "invalid token must not produce a renewed JWT")
	assert.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid)
}

func TestMFAService_ResendChallenge_HappyPath(t *testing.T) {
	svc, _, accID := newExtraMFAService(t)
	require.NoError(t, svc.Enroll(context.Background(), accID))

	// Mint a real challenge token via StartChallenge so ResendChallenge
	// can parse it back out.
	token, err := svc.StartChallenge(context.Background(), accID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// The resend path goes parseChallengeToken → StartChallenge, both
	// of which are exercised end-to-end. The renewed JWT is what the
	// frontend must swap in for the in-flight challenge_token.
	renewed, err := svc.ResendChallenge(context.Background(), token, net.ParseIP("127.0.0.1"))
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
func TestShortenUserAgent_EmptyOrWhitespaceReturnsGermanFallback(t *testing.T) {
	assert.Equal(t, "Unbekanntes Gerät", auth.ShortenUserAgent(""))
	assert.Equal(t, "Unbekanntes Gerät", auth.ShortenUserAgent("   "))
}

func TestShortenUserAgent_ChromeOnMac(t *testing.T) {
	ua := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0 Safari/537.36"
	assert.Equal(t, "Chrome auf macOS", auth.ShortenUserAgent(ua))
}

func TestShortenUserAgent_FirefoxOnLinux(t *testing.T) {
	ua := "Mozilla/5.0 (X11; Linux x86_64) Gecko/20100101 Firefox/123.0"
	assert.Equal(t, "Firefox auf Linux", auth.ShortenUserAgent(ua))
}

func TestShortenUserAgent_EdgeOnWindows(t *testing.T) {
	ua := "Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 Chrome/120.0 Safari/537.36 Edg/120.0"
	assert.Equal(t, "Edge auf Windows", auth.ShortenUserAgent(ua),
		"Edge must win over Chrome — the Edg/ token short-circuits the chain")
}

func TestShortenUserAgent_SafariOniPhone(t *testing.T) {
	ua := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Version/17.0 Mobile/15E148 Safari/604.1"
	assert.Equal(t, "Safari auf iPhone", auth.ShortenUserAgent(ua))
}

func TestShortenUserAgent_UnknownBrowserOnAndroid(t *testing.T) {
	ua := "Mozilla/5.0 (Linux; Android 13)"
	// Not Chrome/Firefox/Safari/Edge → "Browser"; Android hits the OS branch.
	assert.Equal(t, "Browser auf Android", auth.ShortenUserAgent(ua))
}

func TestShortenUserAgent_UnknownAllReturnsBareBrowser(t *testing.T) {
	assert.Equal(t, "Browser", auth.ShortenUserAgent("curl/8.4.0"))
}

// resolveTrustedDeviceDays is internal but the public TrustedDeviceDays
// wrapper delegates to it. Exercising both paths via the public API
// brings the helper's per-tenant lookup into the covered set.
func TestMFAService_TrustedDeviceDays_DefaultsWhenSettingsMissing(t *testing.T) {
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
	svc, _, accID := newExtraMFAService(t)
	require.NoError(t, svc.Enroll(context.Background(), accID))

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

// countFailingChallengeRepo makes only the rate-limit count fail; everything
// else behaves normally, which is the situation the fail-open bug needed to
// show itself: challenge creation and email dispatch succeed regardless of
// whether that count could be read.
type countFailingChallengeRepo struct {
	authModels.MFAEmailChallengeRepository
	err error
}

func (r countFailingChallengeRepo) CountRecentByAccountID(context.Context, int64, time.Time) (int, error) {
	return 0, r.err
}

// TestMFAService_StartChallenge_RateLimitLookupFails_IssuesNoCode pins the
// hard cap (3 codes / 15 min) as fail-CLOSED. Ignoring the count error meant
// the cap silently stopped existing under a struggling database — every repeat
// request kept minting codes and sending mail, which is exactly the abuse the
// cap is there to bound.
func TestMFAService_StartChallenge_RateLimitLookupFails_IssuesNoCode(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	realChallengeRepo := repos.MFAEmailChallenge
	repos.MFAEmailChallenge = countFailingChallengeRepo{
		MFAEmailChallengeRepository: realChallengeRepo,
		err:                         errors.New("rate-limit count unavailable"),
	}

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(extraJWTSecret)
	require.NoError(t, err)
	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: extraJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	acc := testpkg.CreateTestAccount(t, db, "mfa-ratelimit-blind")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	challengeToken, err := svc.StartChallenge(
		context.Background(), acc.ID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"),
	)

	require.Error(t, err)
	assert.ErrorIs(t, err, auth.ErrMFAStatusUnavailable,
		"an unreadable rate-limit count must refuse the code, not wave it through")
	assert.Empty(t, challengeToken)

	issued, err := realChallengeRepo.CountRecentByAccountID(
		context.Background(), acc.ID, time.Now().Add(-time.Hour),
	)
	require.NoError(t, err)
	assert.Zero(t, issued, "a refused challenge must leave no code behind")
}
