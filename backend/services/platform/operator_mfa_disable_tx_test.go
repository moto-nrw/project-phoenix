package platform_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
)

// rollbackTrustedDeviceRecords wraps the real operator MFA records port and
// forces RevokeAllTrustedDevices to return an error. Used to drive the
// Disable cascade to a partial-failure scenario so we can verify the
// transaction rolls back the credential delete that already happened.
type rollbackTrustedDeviceRecords struct {
	platform.OperatorMFARecords
	revokeAllErr error
}

func (r *rollbackTrustedDeviceRecords) RevokeAllTrustedDevices(_ context.Context, _ int64, _ time.Time) error {
	return r.revokeAllErr
}

// failingCredentialRecords wraps the real operator MFA records port and
// fails the enrollment lookup like an unavailable store.
type failingCredentialRecords struct {
	platform.OperatorMFARecords
	err error
}

func (r *failingCredentialRecords) FindCredential(context.Context, int64) (*platformModels.OperatorMFACredential, error) {
	return nil, r.err
}

// TestOperatorMFAService_HasEnrollment_FailsClosedOnStoreError pins the
// login gate: a failed enrollment lookup must refuse the login, never read
// as "not enrolled", which would hand an enrolled operator an enrollment
// token instead of a challenge.
func TestOperatorMFAService_HasEnrollment_FailsClosedOnStoreError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, repos, db := newTestOperatorMFAService(t)
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:     repos,
		Operators: newTestOperatorDirectory(db),
		Records:   &failingCredentialRecords{OperatorMFARecords: newTestOperatorMFARecords(db), err: errors.New("connection reset")},
		TokenAuth: tokenAuth,
		JWTSecret: operatorMFATestJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)
	op := testpkg.CreateTestOperator(t, db)

	enrolled, err := svc.HasEnrollment(ctx, op.ID)
	require.ErrorIs(t, err, authService.ErrMFAStatusUnavailable)
	assert.False(t, enrolled)
}

// TestOperatorMFARecords_StoreFailureIsNotAMissingRow drives the root's
// binding over the real module with an unusable connection: only the
// owner's not-found outcome may become the port's (nil, nil), so a store
// failure must stay an error on every lookup the MFA gate decides on.
func TestOperatorMFARecords_StoreFailureIsNotAMissingRow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	live := testpkg.SetupTestDB(t)
	op := testpkg.CreateTestOperator(t, live)
	db := testpkg.SetupClosableTestDB(t)
	records := newTestOperatorMFARecords(db)
	require.NoError(t, db.Close())

	credential, err := records.FindCredential(ctx, op.ID)
	require.Error(t, err)
	assert.Nil(t, credential)
	challenge, err := records.FindActiveChallenge(ctx, op.ID)
	require.Error(t, err)
	assert.Nil(t, challenge)
	device, err := records.FindActiveTrustedDevice(ctx, op.ID, "token-hash")
	require.Error(t, err)
	assert.Nil(t, device)
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

	// Swap the trusted-device revoke for one that fails.
	realRecords := newTestOperatorMFARecords(db)
	stub := &rollbackTrustedDeviceRecords{
		OperatorMFARecords: realRecords,
		revokeAllErr:       errors.New("simulated DB hiccup during trusted-device revoke"),
	}

	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)
	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:     repos,
		Operators: newTestOperatorDirectory(db),
		Records:   stub,
		TokenAuth: tokenAuth,
		JWTSecret: operatorMFATestJWTSecret,
		DB:        db,
	})
	require.NoError(t, err)
	// Without the runtime the administrative transaction never opens and
	// Disable fails before its first write.
	testpkg.SetTenantRuntime(t, svc, db)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	// Issue a trusted-device cookie via the REAL records (the stub still
	// proxies non-revoke methods to them).
	cookie, _, err := svc.IssueTrustedDevice(ctx, op.ID,
		"Mozilla/5.0 (Test)", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	// Pre-state: credential exists, trusted device exists.
	devicesBefore, err := realRecords.ListActiveTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	require.Len(t, devicesBefore, 1, "fixture: trusted-device row must be present before Disable")
	credRow, err := realRecords.FindCredential(ctx, op.ID)
	require.NoError(t, err)
	require.NotNil(t, credRow, "fixture: credential row must be present before Disable")

	// Force the failure.
	err = svc.Disable(ctx, op.ID)
	require.ErrorIs(t, err, stub.revokeAllErr, "Disable must propagate the simulated revoke error")

	// Post-state: BOTH the credential AND the trusted device must still
	// exist — the transaction rolled back the credential delete that
	// happened before the failing revoke.
	credAfter, err := realRecords.FindCredential(ctx, op.ID)
	require.NoError(t, err, "credential lookup post-Disable should succeed")
	require.NotNil(t, credAfter,
		"credential delete must have rolled back when revoke failed — pre-fix this would be nil")
	assert.Equal(t, credRow.ID, credAfter.ID,
		"the same credential row must still exist (no partial write)")
	verified, err := svc.VerifyTrustedDevice(ctx, op.ID, cookie)
	require.NoError(t, err)
	assert.True(t, verified, "the trusted device must still verify after the rolled-back Disable")
}

// TestOperatorMFAService_Disable_SuccessClearsEverything covers the happy
// path so the new transactional wrapping doesn't accidentally regress the
// successful cascade.
func TestOperatorMFAService_Disable_SuccessClearsEverything(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)
	records := newTestOperatorMFARecords(db)

	op := testpkg.CreateTestOperator(t, db)
	require.NoError(t, svc.Enroll(ctx, op.ID))

	// Build a trusted-device row + bump the lockout counter so all three
	// cascade targets have non-trivial state.
	_, _, err := svc.IssueTrustedDevice(ctx, op.ID, "ua-test", net.ParseIP("203.0.113.21"))
	require.NoError(t, err)
	_, err = newTestOperatorDirectory(db).IncrementMFAAttempts(ctx, op.ID, 5, 15*time.Minute)
	require.NoError(t, err)

	require.NoError(t, svc.Disable(ctx, op.ID))

	// Credential gone: the port reports a missing enrollment as (nil, nil).
	cred, err := records.FindCredential(ctx, op.ID)
	require.NoError(t, err)
	assert.Nil(t, cred, "credential row must be gone after Disable")

	// Trusted devices revoked.
	devices, err := records.ListActiveTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	assert.Empty(t, devices, "trusted devices must be revoked after Disable")

	// Lockout counter cleared.
	opAfter, err := newTestOperatorDirectory(db).FindByID(ctx, op.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, opAfter.MFAAttempts, "mfa_attempts must reset to 0")
	assert.Nil(t, opAfter.MFALockedUntil, "mfa_locked_until must clear")
}
