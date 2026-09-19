package behavior_test

import (
	"context"
	"net"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The disable cascade wipes the enrollment, revokes every trusted device and
// clears the lockout counter in one administrative transaction (PR #1430
// review item #7): a partial failure must not leave the operator with no
// credential while its remember-device cookies still verify.
//
// The rollback itself is driven where the failure can be injected without a
// production seam — `modules/identityaccess/internal/application`'s
// TestOperatorMFADisableRollsBackOnPartialFailure makes the trusted-device
// revoke fail and proves the credential delete is undone with it. This test
// keeps the successful cascade on real rows, so the three writes are proven
// against the tables they run on.
func TestOperatorMFAService_Disable_SuccessClearsEverything(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.EnrollOperatorMFA(ctx, op.ID))

	// Build a trusted-device row and drive the lockout counter up so all
	// three cascade targets have non-trivial state.
	_, _, err := svc.IssueOperatorTrustedDevice(ctx, op.ID, "ua-test", net.ParseIP("203.0.113.21"))
	require.NoError(t, err)
	challenge, err := svc.StartOperatorMFAChallenge(ctx, op.ID, net.ParseIP("203.0.113.21"))
	require.NoError(t, err)
	_, err = svc.VerifyOperatorMFAChallenge(ctx, challenge, "000000")
	require.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid, "fixture: a wrong code must count an attempt")

	require.NoError(t, svc.DisableOperatorMFA(ctx, op.ID))

	// Enrollment gone: a missing credential is the legitimate "not
	// enrolled" answer, not an error.
	enrolled, err := svc.HasOperatorMFAEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.False(t, enrolled, "the credential must be gone after the cascade")

	// Trusted devices revoked.
	devices, err := svc.ListOperatorTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	assert.Empty(t, devices, "trusted devices must be revoked after the cascade")

	// Lockout counter cleared: the operator can start a fresh challenge
	// instead of being refused as locked.
	opAfter := findOperatorRow(t, db, op.ID)
	assert.Equal(t, 0, opAfter.MFAAttempts, "mfa_attempts must reset to 0")
	assert.Nil(t, opAfter.MFALockedUntil, "mfa_locked_until must clear")
}
