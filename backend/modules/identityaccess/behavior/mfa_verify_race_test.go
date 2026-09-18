package behavior_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	modelbase "github.com/moto-nrw/project-phoenix/models/base"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// raceLosingConsume forces the consumption to report the "0 rows affected"
// outcome a concurrent race-loser observes. Every other statement runs for
// real, so the rest of the verify pipeline — the audit write, the lockout
// reset, the foreign keys — sees the genuine row the test created up front.
type raceLosingConsume struct{ markCalls int }

func (r *raceLosingConsume) consume(context.Context, int64, time.Time) error {
	r.markCalls++
	return &modelbase.DatabaseError{
		Op:  "mark mfa email challenge consumed",
		Err: errors.New("expected 1 rows affected, got 0"),
	}
}

// TestMFAService_VerifyChallenge_RaceLoserRejected exercises Item #4 from the
// PR #1430 review: the previous code logged-and-continued when MarkConsumed
// returned "0 rows affected" (the deterministic signal that a concurrent
// verify already consumed the same single-use code) and proceeded to mint a
// VerifiedChallenge. That meant two racing requests both completed login on
// one code, defeating single-use. The fix refuses the loser with
// ErrMFACodeInvalid.
func TestMFAService_VerifyChallenge_RaceLoserRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testpkg.SetupTestDB(t)
	stub := &raceLosingConsume{}
	module := newMFATestModule(t, db, withFailingMFARecords(failingMFARecords{consumeChallenge: stub.consume}))
	svc, repos, tokenAuth := module.MFA, module.Repos, module.TokenAuth
	realChallengeRepo := repos.MFAEmailChallenge

	acc := testpkg.CreateTestAccount(t, db, "mfa-race-loser")

	require.NoError(t, svc.EnrollMFA(ctx, acc.ID))

	// Real StartChallenge writes a real DB row and mints a real JWT — but
	// we never see the plaintext code, so we drive the verify with an
	// arbitrary string. The race-loser branch fires BEFORE the code-hash
	// check would; this test deliberately writes its own challenge with a
	// known hash so VerifyShortCode succeeds and the flow reaches
	// MarkConsumed.
	plaintext := "654321"
	hash, err := securityruntime.HashPassword(plaintext)
	require.NoError(t, err)
	_ = tokenAuth

	now := time.Now()
	challenge := &authmodel.MFAEmailChallenge{
		AccountID: acc.ID,
		Scope:     identityaccess.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: now.Add(identityaccess.MFAChallengeTTL),
		IPAddress: net.ParseIP("203.0.113.99"),
	}
	require.NoError(t, realChallengeRepo.Create(ctx, challenge))
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.mfa_email_challenges").Where("account_id = ?", acc.ID).Exec(context.Background())
	})

	// ChallengeID mirrors what StartChallenge stamps: the verify path resolves
	// the exact row the token names, so a hand-built token must name it too.
	challengeJWT, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:   acc.ID,
		Scope:       identityaccess.MFAChallengeScopeTenant,
		ChallengeID: challenge.ID,
	}, identityaccess.MFAChallengeTTL)
	require.NoError(t, err)

	verified, err := svc.VerifyMFAChallenge(ctx, challengeJWT, plaintext)
	assert.Zero(t, verified, "race-loser must not get a verified challenge")
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid,
		"MarkConsumed-failure must surface as the same generic invalid-code error "+
			"(no info leak distinguishing race-loser from wrong-code attacker)")
	assert.Equal(t, 1, stub.markCalls,
		"VerifyChallenge must call MarkConsumed exactly once before refusing")
}

// TestMFAService_VerifyCodeForAccount_RaceLoserRejected mirrors the previous
// test for the JWT-less code path used by enrollment confirmation. The same
// fail-closed-on-consume-failure rule applies.
func TestMFAService_VerifyCodeForAccount_RaceLoserRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testpkg.SetupTestDB(t)
	stub := &raceLosingConsume{}
	module := newMFATestModule(t, db, withFailingMFARecords(failingMFARecords{consumeChallenge: stub.consume}))
	svc, realChallengeRepo := module.MFA, module.Repos.MFAEmailChallenge

	acc := testpkg.CreateTestAccount(t, db, "mfa-race-loser-jwt-less")

	plaintext := "987654"
	hash, err := securityruntime.HashPassword(plaintext)
	require.NoError(t, err)

	challenge := &authmodel.MFAEmailChallenge{
		AccountID: acc.ID,
		Scope:     identityaccess.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(identityaccess.MFAChallengeTTL),
	}
	require.NoError(t, realChallengeRepo.Create(ctx, challenge))
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.mfa_email_challenges").Where("account_id = ?", acc.ID).Exec(context.Background())
	})

	err = svc.VerifyMFACodeForAccount(ctx, acc.ID, 0, plaintext, identityaccess.MFAChallengeScopeTenant)
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid,
		"VerifyCodeForAccount must refuse the race-loser with the generic invalid-code error")
	assert.Equal(t, 1, stub.markCalls)
}
