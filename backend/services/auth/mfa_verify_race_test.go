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
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	modelbase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// raceLosingChallengeRepo wraps a real MFAEmailChallengeRepository but forces
// MarkConsumed to return a DatabaseError equivalent to the "0 rows affected"
// outcome a concurrent race-loser would observe. Every other method delegates
// to the real implementation so the rest of the verify pipeline (audit
// writes, lockout reset, FK constraints) sees the genuine row a test created
// up front.
type raceLosingChallengeRepo struct {
	authmodel.MFAEmailChallengeRepository
	markCalls int
}

func (r *raceLosingChallengeRepo) MarkConsumed(_ context.Context, id int64, _ time.Time) error {
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

	repos := repositories.NewFactory(db)
	realChallengeRepo := repos.MFAEmailChallenge
	stub := &raceLosingChallengeRepo{MFAEmailChallengeRepository: realChallengeRepo}
	repos.MFAEmailChallenge = stub

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)

	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: testJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	acc := testpkg.CreateTestAccount(t, db, "mfa-race-loser")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	// Real StartChallenge writes a real DB row and mints a real JWT — but
	// we never see the plaintext code, so we drive the verify with an
	// arbitrary string. The race-loser branch fires BEFORE the code-hash
	// check would; this test deliberately writes its own challenge with a
	// known hash so VerifyShortCode succeeds and the flow reaches
	// MarkConsumed.
	plaintext := "654321"
	hash, err := auth.HashShortCode(plaintext)
	require.NoError(t, err)

	now := time.Now()
	challenge := &authmodel.MFAEmailChallenge{
		AccountID: acc.ID,
		Scope:     authjwt.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: now.Add(auth.MFAChallengeTTL),
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
		Scope:       authjwt.MFAChallengeScopeTenant,
		ChallengeID: challenge.ID,
	}, auth.MFAChallengeTTL)
	require.NoError(t, err)

	verified, err := svc.VerifyChallenge(ctx, challengeJWT, plaintext)
	assert.Nil(t, verified, "race-loser must not get a VerifiedChallenge")
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid,
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

	repos := repositories.NewFactory(db)
	realChallengeRepo := repos.MFAEmailChallenge
	stub := &raceLosingChallengeRepo{MFAEmailChallengeRepository: realChallengeRepo}
	repos.MFAEmailChallenge = stub

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)

	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: testJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	acc := testpkg.CreateTestAccount(t, db, "mfa-race-loser-jwt-less")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	plaintext := "987654"
	hash, err := auth.HashShortCode(plaintext)
	require.NoError(t, err)

	challenge := &authmodel.MFAEmailChallenge{
		AccountID: acc.ID,
		Scope:     authjwt.MFAChallengeScopeTenant,
		CodeHash:  hash,
		ExpiresAt: time.Now().Add(auth.MFAChallengeTTL),
	}
	require.NoError(t, realChallengeRepo.Create(ctx, challenge))
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.mfa_email_challenges").Where("account_id = ?", acc.ID).Exec(context.Background())
	})

	err = svc.VerifyCodeForAccount(ctx, acc.ID, 0, plaintext, authjwt.MFAChallengeScopeTenant)
	assert.ErrorIs(t, err, auth.ErrMFACodeInvalid,
		"VerifyCodeForAccount must refuse the race-loser with the generic invalid-code error")
	assert.Equal(t, 1, stub.markCalls)
}
