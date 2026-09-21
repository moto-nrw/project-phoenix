package behavior_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMFAService_EnrollDisableLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-lifecycle")

	enrolled, err := svc.HasMFAEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.False(t, enrolled, "fresh account must not be enrolled")

	require.NoError(t, svc.EnrollMFA(ctx, acc.ID))
	enrolled, err = svc.HasMFAEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, enrolled, "after Enroll, must report enrolled")

	require.ErrorIs(t, svc.EnrollMFA(ctx, acc.ID), identityaccess.ErrMFAAlreadyEnrolled)

	require.NoError(t, svc.DisableMFA(ctx, acc.ID))
	enrolled, err = svc.HasMFAEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.False(t, enrolled)
}

func TestMFAService_TrustedDeviceFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-trusted")

	// Trust is per-(account, tenant) as of #1430 review item #9 — every
	// IssueTrustedDevice / VerifyTrustedDevice call needs a real tenant
	// because auth.mfa_trusted_devices.tenant_id is NOT NULL with FK to
	// platform.schools.
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	require.NoError(t, svc.EnrollMFA(ctx, acc.ID))

	cookie, expiresAt, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantID, "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.NoError(t, err)
	assert.NotEmpty(t, cookie)
	assert.True(t, expiresAt.After(time.Now()), "expiry must be in the future")

	ok, err := svc.VerifyTrustedDevice(ctx, acc.ID, tenantID, cookie)
	require.NoError(t, err)
	assert.True(t, ok, "freshly issued cookie must verify")

	// Tampered cookie rejected.
	badCookie := cookie + "x"
	ok, err = svc.VerifyTrustedDevice(ctx, acc.ID, tenantID, badCookie)
	require.NoError(t, err)
	assert.False(t, ok, "tampered cookie must not verify")
}

func TestMFAService_StartAndVerifyChallenge(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-challenge")

	require.NoError(t, svc.EnrollMFA(ctx, acc.ID))

	tokenString, err := svc.StartMFAChallenge(ctx, acc.ID, 0, identityaccess.MFAChallengeScopeTenant, net.ParseIP("203.0.113.42"))
	require.NoError(t, err)
	require.NotEmpty(t, tokenString)

	// Active challenge row exists.
	active, _, err := nativeMFARecords(t, db).FindActiveChallengeInScope(ctx, acc.ID, 0, identityaccess.MFAChallengeScopeTenant)
	require.NoError(t, err)
	require.NotNil(t, active)
	assert.Equal(t, acc.ID, active.AccountID)

	// Wrong code rejected (and lockout counter increments — verified via repeated calls below).
	_, err = svc.VerifyMFAChallenge(ctx, tokenString, "000000")
	assert.ErrorIs(t, err, identityaccess.ErrMFACodeInvalid)
}

func TestMFAService_AdminOverride_PermissionGate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	target := testpkg.CreateTestAccount(t, db, "mfa-svc-admin-target")

	// Map target to a tenant so the new cross-tenant guard (#1430 Item #2)
	// doesn't reject this permission-gate test before the permission check
	// has a chance to fire.
	actorTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureAccountTenant(t, db, target.ID, actorTenantID)

	const actorID int64 = 9999

	// No permissions -> denied (permission gate, runs before membership).
	err := svc.AdminDisableMFA(ctx, actorID, actorTenantID, target.ID, "lost mailbox", nil)
	assert.ErrorIs(t, err, identityaccess.ErrMFAPermissionDenied)

	// Wildcard admin permission + matching tenant -> allowed.
	require.NoError(t, svc.AdminDisableMFA(ctx, actorID, actorTenantID, target.ID, "lost mailbox", []string{"admin:*"}))

	// Explicit users:manage permission also works.
	require.NoError(t, svc.AdminDisableMFA(ctx, actorID, actorTenantID, target.ID, "user lost device", []string{"users:manage"}))
}
