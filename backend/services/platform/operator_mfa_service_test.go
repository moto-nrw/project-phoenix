package platform_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const operatorMFATestJWTSecret = "test-secret-must-be-at-least-32-chars-long-for-real"

func newTestOperatorMFAService(t *testing.T) (platform.OperatorMFAService, *repositories.Factory, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	tokenAuth, err := authjwt.NewTokenAuthWithSecret(operatorMFATestJWTSecret)
	require.NoError(t, err)

	svc, err := platform.NewOperatorMFAService(platform.OperatorMFAServiceConfig{
		Repos:      repos,
		TokenAuth:  tokenAuth,
		Dispatcher: email.NewDispatcher(testpkg.NewCapturingMailer(), nil),
		JWTSecret:  operatorMFATestJWTSecret,
		DB:         db,
	})
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, svc, db)
	return svc, repos, db
}

func TestOperatorMFAService_EnrollDisableLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	enrolled, err := svc.HasEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)

	require.NoError(t, svc.Enroll(ctx, op.ID))

	enrolled, err = svc.HasEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.True(t, enrolled)

	// Idempotency: a second enrol on the same operator must be rejected.
	require.ErrorIs(t, svc.Enroll(ctx, op.ID), platform.ErrOperatorMFAAlreadyEnrolled)

	require.NoError(t, svc.Disable(ctx, op.ID))

	enrolled, err = svc.HasEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)
}

func TestOperatorMFAService_TrustedDeviceFlow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	require.NoError(t, svc.Enroll(ctx, op.ID))

	cookie, expiresAt, err := svc.IssueTrustedDevice(ctx, op.ID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)
	assert.True(t, expiresAt.After(time.Now()))

	ok, err := svc.VerifyTrustedDevice(ctx, op.ID, cookie)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = svc.VerifyTrustedDevice(ctx, op.ID, cookie+"x")
	require.NoError(t, err)
	assert.False(t, ok, "tampered cookie must not verify")
}

func TestOperatorMFAService_StartChallengeAndWrongCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, repos, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	require.NoError(t, svc.Enroll(ctx, op.ID))

	tokenString, err := svc.StartChallenge(ctx, op.ID, net.ParseIP("203.0.113.42"))
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	active, err := repos.OperatorMFAEmailChallenge.FindActiveByOperatorID(ctx, op.ID)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, op.ID, active.OperatorID)

	// Wrong code rejected.
	_, err = svc.VerifyChallenge(ctx, tokenString, "000000")
	assert.ErrorIs(t, err, platform.ErrOperatorMFACodeInvalid)
}

// TestOperatorMFAService_ListAndRevokeTrustedDevices is the operator-side
// mirror of MFAService.ListTrustedDevices / RevokeTrustedDevice. Same
// semantics: list returns only active rows, revoke removes one device,
// and revoking someone else's device id is rejected.
func TestOperatorMFAService_ListAndRevokeTrustedDevices(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	require.NoError(t, svc.Enroll(ctx, op.ID))

	devices, err := svc.ListTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	assert.Empty(t, devices, "fresh operator has no trusted devices")

	_, _, err = svc.IssueTrustedDevice(ctx, op.ID, "UA-A", net.ParseIP("203.0.113.11"))
	require.NoError(t, err)
	_, _, err = svc.IssueTrustedDevice(ctx, op.ID, "UA-B", net.ParseIP("203.0.113.12"))
	require.NoError(t, err)

	devices, err = svc.ListTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	require.Len(t, devices, 2)

	require.NoError(t, svc.RevokeTrustedDevice(ctx, op.ID, devices[0].ID))

	remaining, err := svc.ListTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.NotEqual(t, devices[0].ID, remaining[0].ID)
}

func TestOperatorMFAService_RevokeTrustedDevice_OwnershipCheck(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	owner := testpkg.CreateTestOperator(t, db)
	attacker := testpkg.CreateTestOperator(t, db)

	require.NoError(t, svc.Enroll(ctx, owner.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, owner.ID, "UA", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)

	ownerDevices, err := svc.ListTrustedDevices(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, ownerDevices, 1)

	// Attacker (different operator) cannot revoke owner's device.
	err = svc.RevokeTrustedDevice(ctx, attacker.ID, ownerDevices[0].ID)
	assert.ErrorIs(t, err, platform.ErrOperatorMFAPermissionDenied)

	stillThere, err := svc.ListTrustedDevices(ctx, owner.ID)
	require.NoError(t, err)
	assert.Len(t, stillThere, 1, "owner's device must not be silently consumed")
}
