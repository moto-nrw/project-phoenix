package platform_test

import (
	"context"
	"errors"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	modelbase "github.com/moto-nrw/project-phoenix/models/base"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// raceLosingOperatorChallengeRepo wraps a real
// OperatorMFAEmailChallengeRepository but forces MarkConsumed to return a
// 0-rows-affected DatabaseError — the exact signal a concurrent race-loser
// would see. Mirror of the tenant-side test helper.
type raceLosingOperatorChallengeRepo struct {
	platformModels.OperatorMFAEmailChallengeRepository
	markCalls int
}

func (r *raceLosingOperatorChallengeRepo) MarkConsumed(_ context.Context, _ int64, _ time.Time) error {
	r.markCalls++
	return &modelbase.DatabaseError{
		Op:  "mark operator mfa email challenge consumed",
		Err: errors.New("expected 1 rows affected, got 0"),
	}
}

// TestOperatorMFAService_VerifyChallenge_RaceLoserRejected exercises Item #4
// (PR #1430 review) on the operator MFA path. Pre-fix, MarkConsumed errors
// were logged and the verify continued — two racing requests both minted
// operator sessions from the same single-use code. The fix refuses the
// loser with the generic ErrOperatorMFACodeInvalid.
func TestOperatorMFAService_VerifyChallenge_RaceLoserRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	// Discard the helper-built service: we need to swap repos.OperatorMFAEmailChallenge
	// before constructing the service so it captures the stub.
	_, repos, db := newTestOperatorMFAService(t)

	realChallengeRepo := repos.OperatorMFAEmailChallenge
	stub := &raceLosingOperatorChallengeRepo{OperatorMFAEmailChallengeRepository: realChallengeRepo}
	repos.OperatorMFAEmailChallenge = stub

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)

	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: operatorMFATestJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	plaintext := "246810"
	hash, err := authService.HashShortCode(plaintext)
	require.NoError(t, err)

	challenge := &platformModels.OperatorMFAEmailChallenge{
		OperatorID: op.ID,
		CodeHash:   hash,
		ExpiresAt:  time.Now().Add(platform.OperatorMFAChallengeTTL),
	}
	require.NoError(t, realChallengeRepo.Create(ctx, challenge))
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("platform.operator_mfa_email_challenges").
			Where("operator_id = ?", op.ID).Exec(context.Background())
	})

	challengeJWT, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: op.ID,
		Scope:     authjwt.MFAChallengeScopePlatform,
	}, platform.OperatorMFAChallengeTTL)
	require.NoError(t, err)

	verified, err := svc.VerifyChallenge(ctx, challengeJWT, plaintext)
	assert.Nil(t, verified, "race-loser must not get a VerifiedChallenge")
	assert.ErrorIs(t, err, platform.ErrOperatorMFACodeInvalid,
		"MarkConsumed-failure must surface as the generic invalid-code error (no info leak)")
	assert.Equal(t, 1, stub.markCalls)
}

// TestOperatorMFAService_VerifyCodeForOperator_RaceLoserRejected mirrors the
// previous test for the JWT-less variant used by enrollment confirmation.
func TestOperatorMFAService_VerifyCodeForOperator_RaceLoserRejected(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, repos, db := newTestOperatorMFAService(t)

	realChallengeRepo := repos.OperatorMFAEmailChallenge
	stub := &raceLosingOperatorChallengeRepo{OperatorMFAEmailChallengeRepository: realChallengeRepo}
	repos.OperatorMFAEmailChallenge = stub

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)

	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:     repos,
		TokenAuth: tokenAuth,
		JWTSecret: operatorMFATestJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)

	op := testpkg.CreateTestOperator(t, db)

	plaintext := "135790"
	hash, err := authService.HashShortCode(plaintext)
	require.NoError(t, err)

	challenge := &platformModels.OperatorMFAEmailChallenge{
		OperatorID: op.ID,
		CodeHash:   hash,
		ExpiresAt:  time.Now().Add(platform.OperatorMFAChallengeTTL),
	}
	require.NoError(t, realChallengeRepo.Create(ctx, challenge))
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("platform.operator_mfa_email_challenges").
			Where("operator_id = ?", op.ID).Exec(context.Background())
	})

	err = svc.VerifyCodeForOperator(ctx, op.ID, plaintext)
	assert.ErrorIs(t, err, platform.ErrOperatorMFACodeInvalid,
		"VerifyCodeForOperator must refuse the race-loser with the generic invalid-code error")
	assert.Equal(t, 1, stub.markCalls)
}
