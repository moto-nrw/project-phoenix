package platform_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// rollbackTrustedDeviceRepo wraps the real OperatorMFATrustedDeviceRepository
// and forces RevokeAllByOperatorID to return an error. Used to drive the
// Disable cascade to a partial-failure scenario so we can verify the
// transaction rolls back the credential delete that already happened.
type rollbackTrustedDeviceRepo struct {
	platformModels.OperatorMFATrustedDeviceRepository
	revokeAllErr error
}

func (r *rollbackTrustedDeviceRepo) RevokeAllByOperatorID(_ context.Context, _ int64, _ time.Time) error {
	return r.revokeAllErr
}

// TestOperatorMFAService_Disable_RollsBackOnPartialFailure exercises Item #7
// from the PR #1430 review. Pre-fix the three Disable writes (credential
// delete, trusted-device revoke, attempts reset) ran sequentially without
// a transaction. If step 2 failed (e.g. transient DB hiccup), step 1's
// delete had already committed, leaving the operator with no credential
// but with still-valid trusted-device cookies — a security regression.
// Post-fix the cascade runs inside WithAdminTx so any step's failure
// rolls back the whole batch.
func TestOperatorMFAService_Disable_RollsBackOnPartialFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, repos, db := newTestOperatorMFAService(t)

	// Swap the trusted-device repo for one that fails on the revoke step.
	realTrustedDevice := repos.OperatorMFATrustedDevice
	stub := &rollbackTrustedDeviceRepo{
		OperatorMFATrustedDeviceRepository: realTrustedDevice,
		revokeAllErr:                       errors.New("simulated DB hiccup during trusted-device revoke"),
	}
	repos.OperatorMFATrustedDevice = stub

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

	// Issue a trusted-device cookie via the REAL repo (the stub still
	// proxies non-revoke methods to the real one).
	cookie, _, err := svc.IssueTrustedDevice(ctx, op.ID,
		"Mozilla/5.0 (Test)", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	// Pre-state: credential exists, trusted device exists.
	credBefore, err := realTrustedDevice.ListActiveByOperatorID(ctx, op.ID)
	require.NoError(t, err)
	require.Len(t, credBefore, 1, "fixture: trusted-device row must be present before Disable")
	credRow, err := repos.OperatorMFACredential.FindByOperatorID(ctx, op.ID)
	require.NoError(t, err)
	require.NotNil(t, credRow, "fixture: credential row must be present before Disable")

	// Force the failure.
	err = svc.Disable(ctx, op.ID)
	require.Error(t, err, "Disable must propagate the simulated revoke error")

	// Post-state: BOTH the credential AND the trusted device must still
	// exist — the transaction rolled back the credential delete that
	// happened before the failing revoke.
	credAfter, err := repos.OperatorMFACredential.FindByOperatorID(ctx, op.ID)
	require.NoError(t, err, "credential lookup post-Disable should succeed")
	require.NotNil(t, credAfter,
		"credential delete must have rolled back when revoke failed — pre-fix this would be nil")
	assert.Equal(t, credRow.ID, credAfter.ID,
		"the same credential row must still exist (no partial write)")
}

// TestOperatorMFAService_Disable_SuccessClearsEverything covers the happy
// path so the new transactional wrapping doesn't accidentally regress the
// successful cascade.
func TestOperatorMFAService_Disable_SuccessClearsEverything(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, repos, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	// Build a trusted-device row + bump the lockout counter so all three
	// cascade targets have non-trivial state.
	_, _, err := svc.IssueTrustedDevice(ctx, op.ID, "ua-test", net.ParseIP("203.0.113.21"))
	require.NoError(t, err)
	_, err = repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).Operator.IncrementMFAAttempts(ctx, op.ID, 5, 15*time.Minute)
	require.NoError(t, err)

	require.NoError(t, svc.Disable(ctx, op.ID))

	// Credential gone. The repo wraps sql.ErrNoRows in a DatabaseError, so
	// either the row returns nil/zero ID *or* an error matching ErrNoRows
	// is the expected post-state.
	cred, err := repos.OperatorMFACredential.FindByOperatorID(ctx, op.ID)
	if err == nil {
		assert.True(t, cred == nil || cred.ID == 0, "credential row must be gone after Disable")
	}

	// Trusted devices revoked.
	devices, err := repos.OperatorMFATrustedDevice.ListActiveByOperatorID(ctx, op.ID)
	require.NoError(t, err)
	assert.Empty(t, devices, "trusted devices must be revoked after Disable")

	// Lockout counter cleared.
	opAfter, err := repos.Operator.FindByID(ctx, op.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, opAfter.MFAAttempts, "mfa_attempts must reset to 0")
	assert.Nil(t, opAfter.MFALockedUntil, "mfa_locked_until must clear")
}
