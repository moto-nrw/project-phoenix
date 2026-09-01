package auth_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type retryingChallengeRepo struct {
	authModels.MFAEmailChallengeRepository
	calls int
}

func (r *retryingChallengeRepo) MarkConsumed(ctx context.Context, id int64, at time.Time) error {
	r.calls++
	if r.calls == 1 {
		return errors.New("temporary cleanup failure")
	}
	return r.MFAEmailChallengeRepository.MarkConsumed(ctx, id, at)
}

type cancelingMFAMailer struct {
	cancel   context.CancelFunc
	err      error
	attempts atomic.Int32
}

func (m *cancelingMFAMailer) Send(message email.Message) error {
	return m.SendContext(context.Background(), message)
}

func (m *cancelingMFAMailer) SendContext(_ context.Context, _ email.Message) error {
	m.attempts.Add(1)
	if m.cancel != nil {
		m.cancel()
	}
	return m.err
}

func newSynchronousDeliveryMFAService(
	t *testing.T,
	mailer email.Mailer,
) (auth.MFAService, *repositories.Factory, *bun.DB, *authjwt.TokenAuth) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(testJWTSecret)
	require.NoError(t, err)
	svc, err := auth.NewMFAService(auth.MFAServiceConfig{
		Repos:      repos,
		TokenAuth:  tokenAuth,
		Dispatcher: email.NewDispatcher(mailer, nil),
		JWTSecret:  testJWTSecret,
		DB:         db,
	})
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, svc, db)
	return svc, repos, db, tokenAuth
}

func TestMFAStartChallengeFailsClosedAndInvalidatesCodeAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	transportErr := errors.New("smtp connection lost")
	mailer := &cancelingMFAMailer{cancel: cancel, err: transportErr}
	svc, repos, db, _ := newSynchronousDeliveryMFAService(t, mailer)
	account := testpkg.CreateTestAccount(t, db, "mfa-sync-delivery-failure")

	token, err := svc.StartChallenge(
		ctx,
		account.ID,
		0,
		authjwt.MFAChallengeScopeTenant,
		net.ParseIP("203.0.113.71"),
	)

	require.ErrorIs(t, err, auth.ErrMFAStatusUnavailable)
	assert.Empty(t, token, "delivery failure must not produce a challenge credential")
	assert.Equal(t, int32(1), mailer.attempts.Load(), "cancellation must stop retries")

	_, activeErr := repos.MFAEmailChallenge.FindActiveByAccountIDInScope(
		context.Background(), account.ID, 0, authjwt.MFAChallengeScopeTenant,
	)
	require.Error(t, activeErr, "the undelivered code must not remain redeemable")

	count, countErr := repos.MFAEmailChallenge.CountRecentByAccountID(
		context.Background(), account.ID, time.Now().Add(-time.Minute),
	)
	require.NoError(t, countErr)
	assert.Equal(t, 1, count, "failed issuance must still count toward the abuse limit")
}

func TestMFAStartChallengeRetriesChallengeInvalidation(t *testing.T) {
	t.Parallel()

	mailer := &cancelingMFAMailer{err: errors.New("smtp connection lost")}
	svc, repos, db, _ := newSynchronousDeliveryMFAService(t, mailer)
	retryingRepo := &retryingChallengeRepo{MFAEmailChallengeRepository: repos.MFAEmailChallenge}
	repos.MFAEmailChallenge = retryingRepo
	account := testpkg.CreateTestAccount(t, db, "mfa-sync-delivery-retry")

	_, err := svc.StartChallenge(context.Background(), account.ID, 0, authjwt.MFAChallengeScopeTenant, nil)

	require.ErrorIs(t, err, auth.ErrMFAStatusUnavailable)
	assert.Equal(t, 2, retryingRepo.calls)
	_, activeErr := retryingRepo.FindActiveByAccountIDInScope(context.Background(), account.ID, 0, authjwt.MFAChallengeScopeTenant)
	require.Error(t, activeErr)
}

func TestMFAResendRejectsExpiredCredentialBeforeDelivery(t *testing.T) {
	t.Parallel()

	mailer := &cancelingMFAMailer{err: errors.New("must not be called")}
	svc, _, db, tokenAuth := newSynchronousDeliveryMFAService(t, mailer)
	account := testpkg.CreateTestAccount(t, db, "mfa-sync-expired-token")
	expiredToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:  account.ID,
		Scope:      authjwt.MFAChallengeScopeTenant,
		MFAPending: true,
	}, -time.Minute)
	require.NoError(t, err)

	renewed, err := svc.ResendChallenge(context.Background(), expiredToken, net.ParseIP("203.0.113.72"))

	require.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid)
	assert.Empty(t, renewed)
	assert.Zero(t, mailer.attempts.Load(), "expired credentials must fail before transport access")
}
