package behavior_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The operator surface renders an unknown or a deactivated operator with 401
// and 403 and everything else with a stable 500. The ceremonies report those
// two outcomes themselves, so the surface keeps classifying them — without
// them a deactivated operator would see a 500 where it used to see a 403.
func TestOperatorPasskeyCeremoniesReportTheOperatorOutcomes(t *testing.T) {
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

	_, err = module.OperatorPasskeys.BeginOperatorPasskeyRegistration(ctx, identityaccess.OperatorPasskeyRegistrationStart{
		OperatorID:     operator.ID,
		ExpectedOrigin: "http://operator.localhost",
		Code:           knownCode,
		Name:           "Test key",
	})
	require.ErrorIs(t, err, identityaccess.ErrOperatorInactive, "a deactivated operator must stay the inactive outcome")

	// A sequence id no operator owns reads as the not-found outcome, which
	// the surface renders as invalid credentials.
	_, err = module.OperatorPasskeys.StartOperatorPasskeyEnrollment(ctx, operator.ID+9_000_000, net.ParseIP("203.0.113.41"))
	require.ErrorIs(t, err, identityaccess.ErrOperatorNotFound, "an unknown operator must stay the not-found outcome")
}

// The operator second factor answers an unknown operator with none of the
// MFA outcomes the operator surface classifies, so that request keeps
// rendering the stable 500 it always did. Only the passkey ceremonies report
// the operator outcomes the surface translates.
func TestOperatorMFAStartChallengeUnknownOperatorStaysUnclassified(t *testing.T) {
	t.Parallel()

	svc, _, _ := newTestOperatorMFAService(t)

	_, err := svc.StartOperatorMFAChallenge(context.Background(), 9_876_543_210, net.ParseIP("203.0.113.42"))
	require.Error(t, err)
	for _, classified := range []error{
		identityaccess.ErrMFAChallengeTokenInvalid,
		identityaccess.ErrMFACodeInvalid,
		identityaccess.ErrMFALocked,
		identityaccess.ErrMFARateLimited,
		identityaccess.ErrMFANotEnrolled,
		identityaccess.ErrMFAAlreadyEnrolled,
		identityaccess.ErrMFAPermissionDenied,
		identityaccess.ErrMFAStatusUnavailable,
	} {
		require.NotErrorIs(t, err, classified,
			"the operator MFA flow never reported an outcome the surface classifies")
	}
}
