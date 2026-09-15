package platform_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type cancelingOperatorMFAMailer struct {
	cancel   context.CancelFunc
	attempts atomic.Int32
}

func (m *cancelingOperatorMFAMailer) Send(message email.Message) error {
	return m.SendContext(context.Background(), message)
}

func (m *cancelingOperatorMFAMailer) SendContext(_ context.Context, _ email.Message) error {
	m.attempts.Add(1)
	m.cancel()
	return errors.New("smtp connection lost")
}

func TestOperatorMFAStartChallengeFailsClosedAndInvalidatesCode(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	mailer := &cancelingOperatorMFAMailer{cancel: cancel}
	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:      repos,
		TokenAuth:  tokenAuth,
		Dispatcher: email.NewDispatcher(mailer, nil),
		JWTSecret:  operatorMFATestJWTSecret,
		DB:         db,
	})
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, svc, db)
	operator := testpkg.CreateTestOperator(t, db)

	token, err := svc.StartChallenge(ctx, operator.ID, net.ParseIP("203.0.113.73"))

	require.ErrorIs(t, err, authService.ErrMFAStatusUnavailable)
	assert.Empty(t, token)
	assert.Equal(t, int32(1), mailer.attempts.Load())
	_, activeErr := repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(context.Background(), operator.ID)
	require.Error(t, activeErr, "the undelivered operator code must not remain redeemable")
}
