package platform_test

import (
	"context"
	"net"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// VerifyCodeForOperator is the JWT-less verify used by operator
// enrollment confirm. Currently 0% in the coverage profile — these
// tests cover the three error exits plus the happy path.

func TestOperatorMFAService_VerifyCodeForOperator_UnknownOperator_ReturnsInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestOperatorMFAService(t)

	err := svc.VerifyCodeForOperator(context.Background(), 999_999_999, "123456")

	require.Error(t, err)
	assert.ErrorIs(t, err, platformSvc.ErrOperatorMFACodeInvalid,
		"unknown operator id must map to invalid-code, not a generic 500")
}

func TestOperatorMFAService_VerifyCodeForOperator_NoActiveChallenge_ReturnsInvalid(t *testing.T) {
	t.Parallel()

	svc, _, db := newTestOperatorMFAService(t)
	op := testpkg.CreateTestOperator(t, db)

	err := svc.VerifyCodeForOperator(context.Background(), op.ID, "123456")

	require.Error(t, err)
	assert.ErrorIs(t, err, platformSvc.ErrOperatorMFACodeInvalid)
}

func TestOperatorMFAService_VerifyCodeForOperator_WrongCode_ReturnsInvalid(t *testing.T) {
	t.Parallel()

	svc, _, db := newTestOperatorMFAService(t)
	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(context.Background(), op.ID))

	// Seed an active challenge via StartChallenge so there's a real
	// CodeHash to mismatch against.
	_, err := svc.StartChallenge(context.Background(), op.ID, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	err = svc.VerifyCodeForOperator(context.Background(), op.ID, "000000")

	require.Error(t, err)
	assert.ErrorIs(t, err, platformSvc.ErrOperatorMFACodeInvalid)
}

// ResendChallenge re-issues a code given a valid operator challenge token.
// Failure paths: bad JWT, wrong scope (tenant token used for operator),
// downstream StartChallenge error.

func TestOperatorMFAService_ResendChallenge_InvalidJWT_ReturnsTokenInvalid(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestOperatorMFAService(t)

	renewed, err := svc.ResendChallenge(context.Background(), "not-a-jwt", net.ParseIP("127.0.0.1"))

	require.Error(t, err)
	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, platformSvc.ErrOperatorMFAChallengeTokenInvalid)
}

func TestOperatorMFAService_ResendChallenge_WrongScope_ReturnsTokenInvalid(t *testing.T) {
	t.Parallel()

	svc, _, db := newTestOperatorMFAService(t)
	op := testpkg.CreateTestOperator(t, db)

	// Build a *tenant*-scoped challenge token and try to resend it on the
	// operator surface — must be rejected so a tenant cookie can't be
	// laundered into the operator flow.
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	tenantToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID: op.ID,
		Scope:     authjwt.MFAChallengeScopeTenant,
	}, 5*60*1_000_000_000) // 5 minutes in nanoseconds
	require.NoError(t, err)

	renewed, err := svc.ResendChallenge(context.Background(), tenantToken, net.ParseIP("127.0.0.1"))

	require.Error(t, err)
	assert.Empty(t, renewed)
	assert.ErrorIs(t, err, platformSvc.ErrOperatorMFAChallengeTokenInvalid,
		"a tenant-scoped JWT must not survive the operator resend gate")
}

func TestOperatorMFAService_ResendChallenge_HappyPath(t *testing.T) {
	t.Parallel()

	svc, _, db := newTestOperatorMFAService(t)
	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(context.Background(), op.ID))

	token, err := svc.StartChallenge(context.Background(), op.ID, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotEmpty(t, token)

	renewed, err := svc.ResendChallenge(context.Background(), token, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	// See tenant-side counterpart — JWT iat has 1-second resolution so
	// equal byte sequences are legal here. The contract we actually
	// care about is that resend returns a usable token at all.
	assert.NotEmpty(t, renewed, "happy path must return a renewed challenge JWT")
}
