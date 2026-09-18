package auth_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A challenge token names the exact auth.mfa_email_challenges row it was
// minted for. Before that binding, verify resolved "the newest active code for
// this account", so two challenges in flight for one person — a tenant login
// and a school login, say — could redeem each other's code: the scope check
// only inspects the JWT, and the code comparison then ran against whichever
// row happened to be newest.

// seedChallenge writes an active challenge with a known code and returns the
// row, so a test can drive verify with the plaintext.
func seedChallenge(t *testing.T, repo authmodel.MFAEmailChallengeRepository, accountID int64, code string, ttl time.Duration) *authmodel.MFAEmailChallenge {
	t.Helper()
	hash, err := auth.HashPassword(code)
	require.NoError(t, err)
	challenge := &authmodel.MFAEmailChallenge{
		AccountID: accountID,
		Scope:     auth.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(ttl),
		IPAddress: net.ParseIP("203.0.113.10"),
	}
	require.NoError(t, repo.Create(context.Background(), challenge))
	return challenge
}

func TestStartChallenge_TokenCarriesItsChallengeID(t *testing.T) {
	t.Parallel()

	scenario := newMFATestScenario(t)
	svc, repos, accID := scenario.MFA, scenario.Repos, scenario.AccountID
	ctx := context.Background()
	require.NoError(t, svc.EnrollMFA(ctx, accID))

	token, err := svc.StartMFAChallenge(ctx, accID, 0, auth.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	tokenAuth := scenario.TokenAuth
	claims, err := tokenAuth.ParseMFAChallengeJWT(token)
	require.NoError(t, err)

	persisted, err := repos.MFAEmailChallenge.FindActiveByAccountIDInScope(ctx, accID, 0, auth.MFAChallengeScopeTenant)
	require.NoError(t, err)
	assert.Equal(t, persisted.ID, claims.ChallengeID,
		"the challenge token must name the row it was minted for")
}

func TestVerifyChallenge_RedeemsOnlyTheNamedChallenge(t *testing.T) {
	t.Parallel()

	scenario := newMFATestScenario(t)
	svc, repos, accID := scenario.MFA, scenario.Repos, scenario.AccountID
	ctx := context.Background()

	tokenAuth := scenario.TokenAuth

	// Two challenges in flight for one account. Distinct expiries because the
	// partial unique index allows only one unconsumed row per (account,
	// expires_at).
	older := seedChallenge(t, repos.MFAEmailChallenge, accID, "111111", identityaccess.MFAChallengeTTL)
	newer := seedChallenge(t, repos.MFAEmailChallenge, accID, "222222", identityaccess.MFAChallengeTTL+time.Minute)
	t.Cleanup(func() {
		_, _ = repos.MFAEmailChallenge.DeleteExpired(context.Background())
	})
	require.NotEqual(t, older.ID, newer.ID)

	olderToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:   accID,
		Scope:       auth.MFAChallengeScopeTenant,
		ChallengeID: older.ID,
	}, identityaccess.MFAChallengeTTL)
	require.NoError(t, err)

	// The other challenge's code must not satisfy this token. Under the old
	// "newest active row" lookup this call succeeded.
	verified, err := svc.VerifyMFAChallenge(ctx, olderToken, "222222")
	assert.Zero(t, verified, "a refused verification must not name an account")
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid,
		"a code belonging to a different challenge must not be redeemable here")

	// Its own code still works.
	verified, err = svc.VerifyMFAChallenge(ctx, olderToken, "111111")
	require.NoError(t, err)
	require.NotNil(t, verified)
	assert.Equal(t, accID, verified.AccountID)
}

func TestVerifyChallenge_TokenWithoutChallengeID_Refused(t *testing.T) {
	t.Parallel()

	// Tokens minted before the binding existed carry no challenge_id. They are
	// refused rather than falling back to the ambiguous lookup — the challenge
	// TTL is 5 minutes, so the whole cost is one re-login inside the deploy
	// window.
	scenario := newMFATestScenario(t)
	svc, repos, accID := scenario.MFA, scenario.Repos, scenario.AccountID
	ctx := context.Background()

	seedChallenge(t, repos.MFAEmailChallenge, accID, "333333", identityaccess.MFAChallengeTTL)
	t.Cleanup(func() {
		_, _ = repos.MFAEmailChallenge.DeleteExpired(context.Background())
	})

	tokenAuth := scenario.TokenAuth
	legacyToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: accID,
		Scope:     auth.MFAChallengeScopeTenant,
	}, identityaccess.MFAChallengeTTL)
	require.NoError(t, err)

	verified, err := svc.VerifyMFAChallenge(ctx, legacyToken, "333333")

	assert.Zero(t, verified, "a refused verification must not name an account")
	assert.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid,
		"a token that names no challenge row must be refused, not resolved by guesswork")
}

func TestResendChallengeForScope_ForeignScope_Refused(t *testing.T) {
	t.Parallel()

	// The scope guard the tenant and school resend endpoints both rely on: a
	// challenge started for one portal must not be re-driven — and its
	// 3-codes-per-15-minutes budget burned — through another portal's surface.
	scenario := newMFATestScenario(t)
	svc, accID := scenario.MFA, scenario.AccountID
	ctx := context.Background()
	require.NoError(t, svc.EnrollMFA(ctx, accID))

	schoolToken, err := svc.StartMFAChallenge(ctx, accID, 0, auth.MFAChallengeScopeSchool, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	renewed, err := svc.ResendMFAChallengeForScope(ctx, schoolToken, net.ParseIP("127.0.0.1"), auth.MFAChallengeScopeTenant)

	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, auth.ErrMFAUnsupportedScope,
		"the tenant resend surface must refuse a school-scope challenge")
}

func TestVerifyChallengeForOwner_ForeignChallengeRefusedAndLeftRedeemable(t *testing.T) {
	t.Parallel()

	// The school enrollment confirm arrives with TWO credentials: an
	// enrollment token naming account + school, and a challenge token. When
	// they disagree the request is refused either way — but the refusal must
	// happen BEFORE the code comparison, because verification consumes the
	// challenge. Comparing afterwards (what the handler used to do with the
	// returned VerifiedChallenge) burned a stranger's challenge on the way to
	// the 401, killing their in-flight login with a code that had just gone
	// "invalid".
	scenario := newMFATestScenario(t)
	svc, repos, victimID := scenario.MFA, scenario.Repos, scenario.AccountID
	ctx := context.Background()

	db := testpkg.SetupTestDB(t)
	caller := testpkg.CreateTestAccount(t, db, "mfa-owner-mismatch")
	require.NotEqual(t, victimID, caller.ID)

	victimChallenge := seedChallenge(t, repos.MFAEmailChallenge, victimID, "424242", identityaccess.MFAChallengeTTL)
	t.Cleanup(func() {
		_, _ = repos.MFAEmailChallenge.DeleteExpired(context.Background())
	})

	tokenAuth := scenario.TokenAuth
	victimToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:   victimID,
		Scope:       auth.MFAChallengeScopeTenant,
		ChallengeID: victimChallenge.ID,
	}, identityaccess.MFAChallengeTTL)
	require.NoError(t, err)

	// Right token, right code, wrong owner.
	verified, err := svc.VerifyMFAChallengeForOwner(
		ctx, victimToken, "424242", auth.MFAChallengeScopeTenant, caller.ID, 0)

	assert.Zero(t, verified, "a refused verification must not name an account")
	assert.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid,
		"a challenge belonging to another account must be refused")

	stillActive, err := repos.MFAEmailChallenge.FindActiveByIDForAccount(ctx, victimChallenge.ID, victimID)
	require.NoError(t, err)
	require.NotNil(t, stillActive,
		"the refused challenge must not have been consumed — it belongs to someone else's login")

	// And its owner can still redeem it.
	verified, err = svc.VerifyMFAChallengeForOwner(
		ctx, victimToken, "424242", auth.MFAChallengeScopeTenant, victimID, 0)
	require.NoError(t, err)
	require.NotNil(t, verified)
	assert.Equal(t, victimID, verified.AccountID)
}
