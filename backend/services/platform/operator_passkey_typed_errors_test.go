package platform_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator surface renders an unknown or a deactivated operator from two
// typed errors, not from a sentinel: AuthErrorRenderer answers them with 401
// and 403 and everything else with a stable 500. The ceremonies moved into
// Identity & Access with #3331, so the composition translates the module's
// outcomes back into those shapes — without it a deactivated operator would
// see a 500 where it used to see a 403.
func TestOperatorPasskeyCeremoniesKeepTheTypedOperatorErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := testpkg.SetupTestDB(t)
	module := newOperatorMFATestModule(t, db, withDeliveringMailer(nil)...)
	require.NotNil(t, module.OperatorPasskeys)

	operator := testpkg.CreateTestOperator(t, db)
	deleteOperatorChallenges(t, db, operator.ID)
	require.NoError(t, module.OperatorMFA.EnrollOperatorMFA(ctx, operator.ID))
	_, err := module.OperatorMFA.StartOperatorMFAChallenge(ctx, operator.ID, net.ParseIP("203.0.113.40"))
	require.NoError(t, err)
	active := activeOperatorChallenge(t, db, operator.ID)
	require.NotNil(t, active)

	const knownCode = "515151"
	substituteOperatorChallengeCode(t, db, active.ID, knownCode)

	// The registration verifies the code before it reads the operator, so the
	// deactivation must be the reason the ceremony refuses.
	_, err = db.NewUpdate().
		Table("platform.operators").
		Set("active = false").
		Where("id = ?", operator.ID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = module.OperatorPasskeys.BeginOperatorPasskeyRegistration(ctx, platform.OperatorPasskeyRegistrationStart{
		OperatorID:     operator.ID,
		ExpectedOrigin: "http://operator.localhost",
		Code:           knownCode,
		Name:           "Test key",
	})
	var inactive *platform.OperatorInactiveError
	require.ErrorAs(t, err, &inactive, "a deactivated operator must stay the typed inactive error")
	assert.Equal(t, operator.ID, inactive.OperatorID)

	// A sequence id no operator owns reads as the typed not-found error, which
	// the surface renders as invalid credentials.
	_, err = module.OperatorPasskeys.StartOperatorPasskeyEnrollment(ctx, operator.ID+9_000_000, net.ParseIP("203.0.113.41"))
	var notFound *platform.OperatorNotFoundError
	require.ErrorAs(t, err, &notFound, "an unknown operator must stay the typed not-found error")
}

// The operator second factor answered an unknown operator with a plain error
// before the move and still does: only the passkey ceremonies carry the typed
// shapes, and the surface maps everything else to its stable message.
func TestOperatorMFAStartChallengeUnknownOperatorStaysUntyped(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestOperatorMFAService(t)

	_, err := svc.StartOperatorMFAChallenge(context.Background(), 9_876_543_210, net.ParseIP("203.0.113.42"))
	require.Error(t, err)
	var notFound *platform.OperatorNotFoundError
	assert.False(t, errors.As(err, &notFound),
		"the operator MFA flow never produced the typed error")
}
