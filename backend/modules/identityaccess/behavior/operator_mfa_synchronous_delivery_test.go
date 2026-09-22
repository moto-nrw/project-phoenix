package behavior_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// refusingOperatorMFAMailer refuses every message, the way a dead SMTP hop
// does.
type refusingOperatorMFAMailer struct {
	attempts atomic.Int32
}

func (m *refusingOperatorMFAMailer) Send(message testpkg.EmailMessage) error {
	return m.SendContext(context.Background(), message)
}

func (m *refusingOperatorMFAMailer) SendContext(_ context.Context, _ testpkg.EmailMessage) error {
	m.attempts.Add(1)
	return errors.New("smtp connection lost")
}

// The operator code is sent synchronously and the row is only made
// redeemable once the transport accepted it. A refused send must therefore
// fail the login attempt and leave no code behind that a second request
// could redeem.
func TestOperatorMFAStartChallengeFailsClosedAndInvalidatesCode(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	mailer := &refusingOperatorMFAMailer{}
	module := newOperatorMFATestModule(t, db,
		services.WithAuthTestMailer(mailer),
		services.WithAuthTestMFABackoff(time.Millisecond))
	operator := testpkg.CreateTestOperator(t, db)
	deleteOperatorChallenges(t, db, operator.ID)

	token, err := module.OperatorMFA.StartOperatorMFAChallenge(
		context.Background(), operator.ID, net.ParseIP("203.0.113.73"))

	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable)
	assert.Empty(t, token)
	assert.Positive(t, mailer.attempts.Load(), "the code is sent synchronously, so the attempt is visible here")
	assert.Nil(t, activeOperatorChallenge(t, db, operator.ID),
		"the undelivered operator code must not remain redeemable")
}
