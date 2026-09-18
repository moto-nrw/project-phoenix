package auth_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	auth "github.com/moto-nrw/project-phoenix/services/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

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
	module := newMFATestModule(t, db,
		services.WithAuthTestMailer(mailer),
		// One attempt: the contract under test is that a refused transport
		// fails the issuance, not how long the retries take.
		services.WithAuthTestMFABackoff(time.Millisecond),
	)
	return module.MFA, module.Repos, db, module.TokenAuth
}

func TestMFAStartChallengeFailsClosedAndInvalidatesCodeAfterCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	transportErr := errors.New("smtp connection lost")
	mailer := &cancelingMFAMailer{cancel: cancel, err: transportErr}
	svc, repos, db, _ := newSynchronousDeliveryMFAService(t, mailer)
	account := testpkg.CreateTestAccount(t, db, "mfa-sync-delivery-failure")

	token, err := svc.StartMFAChallenge(
		ctx,
		account.ID,
		0,
		auth.MFAChallengeScopeTenant,
		net.ParseIP("203.0.113.71"),
	)

	require.ErrorIs(t, err, auth.ErrMFAStatusUnavailable)
	assert.Empty(t, token, "delivery failure must not produce a challenge credential")
	assert.Equal(t, int32(1), mailer.attempts.Load(), "cancellation must stop retries")

	_, activeErr := repos.MFAEmailChallenge.FindActiveByAccountIDInScope(
		context.Background(), account.ID, 0, auth.MFAChallengeScopeTenant,
	)
	require.Error(t, activeErr, "the undelivered code must not remain redeemable")

	count, countErr := repos.MFAEmailChallenge.CountRecentByAccountID(
		context.Background(), account.ID, time.Now().Add(-time.Minute),
	)
	require.NoError(t, countErr)
	assert.Equal(t, 1, count, "failed issuance must still count toward the abuse limit")
}

func TestMFAResendRejectsExpiredCredentialBeforeDelivery(t *testing.T) {
	t.Parallel()

	mailer := &cancelingMFAMailer{err: errors.New("must not be called")}
	svc, _, db, tokenAuth := newSynchronousDeliveryMFAService(t, mailer)
	account := testpkg.CreateTestAccount(t, db, "mfa-sync-expired-token")
	expiredToken, err := tokenAuth.CreateMFAChallengeJWT(authjwt.MFAChallengeClaims{
		AccountID:  account.ID,
		Scope:      auth.MFAChallengeScopeTenant,
		MFAPending: true,
	}, -time.Minute)
	require.NoError(t, err)

	renewed, err := svc.ResendMFAChallenge(context.Background(), expiredToken, net.ParseIP("203.0.113.72"))

	require.ErrorIs(t, err, auth.ErrMFAChallengeTokenInvalid)
	assert.Empty(t, renewed)
	assert.Zero(t, mailer.attempts.Load(), "expired credentials must fail before transport access")
}
