package platform_test

import (
	"context"
	"net"
	"testing"
	"time"

	authSvc "github.com/moto-nrw/project-phoenix/services/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestOperatorMFAService_EnrollDisableLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	enrolled, err := svc.HasOperatorMFAEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)

	require.NoError(t, svc.EnrollOperatorMFA(ctx, op.ID))

	enrolled, err = svc.HasOperatorMFAEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.True(t, enrolled)

	// Idempotency: a second enrol on the same operator must be rejected.
	require.ErrorIs(t, svc.EnrollOperatorMFA(ctx, op.ID), authSvc.ErrMFAAlreadyEnrolled)

	require.NoError(t, svc.DisableOperatorMFA(ctx, op.ID))

	enrolled, err = svc.HasOperatorMFAEnrollment(ctx, op.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)
}

func TestOperatorMFAService_TrustedDeviceFlow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	require.NoError(t, svc.EnrollOperatorMFA(ctx, op.ID))

	cookie, expiresAt, err := svc.IssueOperatorTrustedDevice(ctx, op.ID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)
	assert.True(t, expiresAt.After(time.Now()))

	ok, err := svc.VerifyOperatorTrustedDevice(ctx, op.ID, cookie)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = svc.VerifyOperatorTrustedDevice(ctx, op.ID, cookie+"x")
	require.NoError(t, err)
	assert.False(t, ok, "tampered cookie must not verify")
}

func TestOperatorMFAService_StartChallengeAndWrongCode(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc, _, db := newTestOperatorMFAService(t)

	op := testpkg.CreateTestOperator(t, db)

	require.NoError(t, svc.EnrollOperatorMFA(ctx, op.ID))

	tokenString, err := svc.StartOperatorMFAChallenge(ctx, op.ID, net.ParseIP("203.0.113.42"))
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	active := activeOperatorChallenge(t, db, op.ID)
	require.NotNil(t, active, "a delivered code must leave a redeemable row")
	assert.Equal(t, op.ID, active.OperatorID)

	// Wrong code rejected.
	_, err = svc.VerifyOperatorMFAChallenge(ctx, tokenString, "000000")
	assert.ErrorIs(t, err, authSvc.ErrMFACodeInvalid)
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

	require.NoError(t, svc.EnrollOperatorMFA(ctx, op.ID))

	devices, err := svc.ListOperatorTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	assert.Empty(t, devices, "fresh operator has no trusted devices")

	_, _, err = svc.IssueOperatorTrustedDevice(ctx, op.ID, "UA-A", net.ParseIP("203.0.113.11"))
	require.NoError(t, err)
	_, _, err = svc.IssueOperatorTrustedDevice(ctx, op.ID, "UA-B", net.ParseIP("203.0.113.12"))
	require.NoError(t, err)

	devices, err = svc.ListOperatorTrustedDevices(ctx, op.ID)
	require.NoError(t, err)
	require.Len(t, devices, 2)

	require.NoError(t, svc.RevokeOperatorTrustedDeviceOwned(ctx, op.ID, devices[0].ID))

	remaining, err := svc.ListOperatorTrustedDevices(ctx, op.ID)
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

	require.NoError(t, svc.EnrollOperatorMFA(ctx, owner.ID))
	_, _, err := svc.IssueOperatorTrustedDevice(ctx, owner.ID, "UA", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)

	ownerDevices, err := svc.ListOperatorTrustedDevices(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, ownerDevices, 1)

	// Attacker (different operator) cannot revoke owner's device.
	err = svc.RevokeOperatorTrustedDeviceOwned(ctx, attacker.ID, ownerDevices[0].ID)
	assert.ErrorIs(t, err, authSvc.ErrMFAPermissionDenied)

	stillThere, err := svc.ListOperatorTrustedDevices(ctx, owner.ID)
	require.NoError(t, err)
	assert.Len(t, stillThere, 1, "owner's device must not be silently consumed")
}
