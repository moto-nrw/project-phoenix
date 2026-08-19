package auth_test

import (
	"context"
	"net"
	"testing"
	"time"

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
	hash, err := auth.HashShortCode(code)
	require.NoError(t, err)
	challenge := &authmodel.MFAEmailChallenge{
		AccountID: accountID,
		Scope:     authjwt.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(ttl),
		IPAddress: net.ParseIP("203.0.113.10"),
	}
	require.NoError(t, repo.Create(context.Background(), challenge))
	return challenge
}

func TestStartChallenge_TokenCarriesItsChallengeID(t *testing.T) {
	svc, repos, accID := newExtraMFAService(t)
	ctx := context.Background()
	require.NoError(t, svc.Enroll(ctx, accID))

	token, err := svc.StartChallenge(ctx, accID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(extraJWTSecret)
	require.NoError(t, err)
	claims, err := tokenAuth.ParseMFAChallengeJWT(token)
	require.NoError(t, err)

	persisted, err := repos.MFAEmailChallenge.FindActiveByAccountIDInScope(ctx, accID, 0, authjwt.MFAChallengeScopeTenant)
	require.NoError(t, err)
	assert.Equal(t, persisted.ID, claims.ChallengeID,
		"the challenge token must name the row it was minted for")
}

func TestVerifyChallenge_RedeemsOnlyTheNamedChallenge(t *testing.T) {
	svc, repos, accID := newExtraMFAService(t)
	ctx := context.Background()

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(extraJWTSecret)
	require.NoError(t, err)

	// Two challenges in flight for one account. Distinct expiries because the
	// partial unique index allows only one unconsumed row per (account,
	// expires_at).
	older := seedChallenge(t, repos.MFAEmailChallenge, accID, "111111", auth.MFAChallengeTTL)
	newer := seedChallenge(t, repos.MFAEmailChallenge, accID, "222222", auth.MFAChallengeTTL+time.Minute)
	t.Cleanup(func() {
		_, _ = repos.MFAEmailChallenge.DeleteExpired(context.Background())
	})
	require.NotEqual(t, older.ID, newer.ID)

	olderToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:   accID,
		Scope:       authjwt.MFAChallengeScopeTenant,
		ChallengeID: older.ID,
	}, auth.MFAChallengeTTL)
	require.NoError(t, err)

	// The other challenge's code must not satisfy this token. Under the old
	// "newest active row" lookup this call succeeded.
	verified, err := svc.VerifyChallenge(ctx, olderToken, "222222")
	assert.Nil(t, verified)
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid,
		"a code belonging to a different challenge must not be redeemable here")

	// Its own code still works.
	verified, err = svc.VerifyChallenge(ctx, olderToken, "111111")
	require.NoError(t, err)
	require.NotNil(t, verified)
	assert.Equal(t, accID, verified.AccountID)
}

func TestVerifyChallenge_TokenWithoutChallengeID_Refused(t *testing.T) {
	// Tokens minted before the binding existed carry no challenge_id. They are
	// refused rather than falling back to the ambiguous lookup — the challenge
	// TTL is 5 minutes, so the whole cost is one re-login inside the deploy
	// window.
	svc, repos, accID := newExtraMFAService(t)
	ctx := context.Background()

	seedChallenge(t, repos.MFAEmailChallenge, accID, "333333", auth.MFAChallengeTTL)
	t.Cleanup(func() {
		_, _ = repos.MFAEmailChallenge.DeleteExpired(context.Background())
	})

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(extraJWTSecret)
	require.NoError(t, err)
	legacyToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: accID,
		Scope:     authjwt.MFAChallengeScopeTenant,
	}, auth.MFAChallengeTTL)
	require.NoError(t, err)

	verified, err := svc.VerifyChallenge(ctx, legacyToken, "333333")

	assert.Nil(t, verified)
	assert.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid,
		"a token that names no challenge row must be refused, not resolved by guesswork")
}

func TestResendChallengeForScope_ForeignScope_Refused(t *testing.T) {
	// The scope guard the tenant and school resend endpoints both rely on: a
	// challenge started for one portal must not be re-driven — and its
	// 3-codes-per-15-minutes budget burned — through another portal's surface.
	svc, _, accID := newExtraMFAService(t)
	ctx := context.Background()
	require.NoError(t, svc.Enroll(ctx, accID))

	schoolToken, err := svc.StartChallenge(ctx, accID, 0, authjwt.MFAChallengeScopeSchool, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	renewed, err := svc.ResendChallengeForScope(ctx, schoolToken, net.ParseIP("127.0.0.1"), authjwt.MFAChallengeScopeTenant)

	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, auth.ErrMFAUnsupportedScope,
		"the tenant resend surface must refuse a school-scope challenge")
}

func TestVerifyChallengeForOwner_ForeignChallengeRefusedAndLeftRedeemable(t *testing.T) {
	// The school enrollment confirm arrives with TWO credentials: an
	// enrollment token naming account + school, and a challenge token. When
	// they disagree the request is refused either way — but the refusal must
	// happen BEFORE the code comparison, because verification consumes the
	// challenge. Comparing afterwards (what the handler used to do with the
	// returned VerifiedChallenge) burned a stranger's challenge on the way to
	// the 401, killing their in-flight login with a code that had just gone
	// "invalid".
	svc, repos, victimID := newExtraMFAService(t)
	ctx := context.Background()

	db := testpkg.SetupTestDB(t)
	caller := testpkg.CreateTestAccount(t, db, "mfa-owner-mismatch")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, caller.ID) })
	require.NotEqual(t, victimID, caller.ID)

	victimChallenge := seedChallenge(t, repos.MFAEmailChallenge, victimID, "424242", auth.MFAChallengeTTL)
	t.Cleanup(func() {
		_, _ = repos.MFAEmailChallenge.DeleteExpired(context.Background())
	})

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(extraJWTSecret)
	require.NoError(t, err)
	victimToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:   victimID,
		Scope:       authjwt.MFAChallengeScopeTenant,
		ChallengeID: victimChallenge.ID,
	}, auth.MFAChallengeTTL)
	require.NoError(t, err)

	// Right token, right code, wrong owner.
	verified, err := svc.VerifyChallengeForOwner(
		ctx, victimToken, "424242", authjwt.MFAChallengeScopeTenant, caller.ID, 0)

	assert.Nil(t, verified)
	assert.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid,
		"a challenge belonging to another account must be refused")

	stillActive, err := repos.MFAEmailChallenge.FindActiveByIDForAccount(ctx, victimChallenge.ID, victimID)
	require.NoError(t, err)
	require.NotNil(t, stillActive,
		"the refused challenge must not have been consumed — it belongs to someone else's login")

	// And its owner can still redeem it.
	verified, err = svc.VerifyChallengeForOwner(
		ctx, victimToken, "424242", authjwt.MFAChallengeScopeTenant, victimID, 0)
	require.NoError(t, err)
	require.NotNil(t, verified)
	assert.Equal(t, victimID, verified.AccountID)
}
