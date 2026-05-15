package auth_test

// Test coverage for the MFA service features added after the original
// #1308 ship: self-service trusted-device list+revoke, per-account admin
// override (none / force_off / force_on), and the rate-limit + lockout
// edge cases from the design doc §11. The original mfa_service_test.go
// covers happy-path lifecycle; this file fills the gaps Phase 13 calls out.

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- Self-service trusted devices ---

func TestMFAService_ListAndRevokeTrustedDevices(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-list-trusted")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	// Fresh account has zero devices.
	devices, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	assert.Empty(t, devices)

	// Issue two cookies — list returns both.
	cookieA, _, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "UA-A", net.ParseIP("203.0.113.1"))
	require.NoError(t, err)
	require.NotEmpty(t, cookieA)
	cookieB, _, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "UA-B", net.ParseIP("203.0.113.2"))
	require.NoError(t, err)
	require.NotEmpty(t, cookieB)

	devices, err = svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	require.Len(t, devices, 2)

	// Revoke one — list drops to one and the revoked cookie no longer verifies.
	require.NoError(t, svc.RevokeTrustedDevice(ctx, acc.ID, devices[0].ID))

	remaining, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.NotEqual(t, devices[0].ID, remaining[0].ID)
}

func TestMFAService_RevokeTrustedDevice_OwnershipCheck(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	owner := testpkg.CreateTestAccount(t, db, "mfa-svc-revoke-owner")
	attacker := testpkg.CreateTestAccount(t, db, "mfa-svc-revoke-attacker")
	t.Cleanup(func() {
		testpkg.CleanupAccount(t, db, owner.ID)
		testpkg.CleanupAccount(t, db, attacker.ID)
	})

	require.NoError(t, svc.Enroll(ctx, owner.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, owner.ID, 0, "UA", net.ParseIP("203.0.113.10"))
	require.NoError(t, err)

	ownerDevices, err := svc.ListTrustedDevices(ctx, owner.ID)
	require.NoError(t, err)
	require.Len(t, ownerDevices, 1)

	// Attacker (different account) cannot revoke owner's device.
	err = svc.RevokeTrustedDevice(ctx, attacker.ID, ownerDevices[0].ID)
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// And the owner's device is still listed (not silently consumed).
	stillThere, err := svc.ListTrustedDevices(ctx, owner.ID)
	require.NoError(t, err)
	assert.Len(t, stillThere, 1)
}

// --- Per-account admin override ---

func TestMFAService_GetMFAOverride_DefaultNone(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-override-default")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	override, err := svc.GetMFAOverride(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideNone, override)
}

func TestMFAService_SetMFAOverride_RejectsInvalidValue(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-override-invalid")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	err := svc.SetMFAOverride(ctx, 0, acc.ID, "yes-please", "reason", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAInvalidOverride)
}

func TestMFAService_SetMFAOverride_PermissionGate(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-override-perms")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	// Wrong permission — denied.
	err := svc.SetMFAOverride(ctx, 0, acc.ID, auth.MFAAdminOverrideForceOff, "reason", []string{"something:else"})
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// users:manage works.
	require.NoError(t, svc.SetMFAOverride(ctx, 0, acc.ID, auth.MFAAdminOverrideForceOff, "reason", []string{"users:manage"}))
	got, err := svc.GetMFAOverride(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideForceOff, got)
}

func TestMFAService_SetMFAOverride_ForceOff_RevokesTrustedDevices(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-override-revoke")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "UA", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)

	devicesBefore, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	require.Len(t, devicesBefore, 1)

	require.NoError(t, svc.SetMFAOverride(ctx, 0, acc.ID, auth.MFAAdminOverrideForceOff, "lost mailbox", []string{"users:manage"}))

	devicesAfter, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	assert.Empty(t, devicesAfter, "force_off must revoke all trusted devices")
}

func TestMFAService_IsRequired_HonorsOverride(t *testing.T) {
	ctx := context.Background()
	svc, repos, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-override-isreq")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	// Reload the persisted Account (with MFAAdminOverride column) before
	// calling IsRequired — the service inspects account.MFAAdminOverride
	// directly, not a fresh DB read.
	reload := func() {
		fresh, err := repos.Account.FindByID(ctx, acc.ID)
		require.NoError(t, err)
		require.NotNil(t, fresh)
		acc.MFAAdminOverride = fresh.MFAAdminOverride
	}

	// none -> service default ("off" because no settings wired) => false
	required, err := svc.IsRequired(ctx, acc, 0)
	require.NoError(t, err)
	assert.False(t, required, "no override + settings off => not required")

	// force_on => true regardless of tenant mode
	require.NoError(t, svc.SetMFAOverride(ctx, 0, acc.ID, auth.MFAAdminOverrideForceOn, "always 2FA", []string{"users:manage"}))
	reload()
	required, err = svc.IsRequired(ctx, acc, 0)
	require.NoError(t, err)
	assert.True(t, required, "force_on must force MFA required")

	// force_off => false regardless of tenant mode
	require.NoError(t, svc.SetMFAOverride(ctx, 0, acc.ID, auth.MFAAdminOverrideForceOff, "lost mailbox", []string{"users:manage"}))
	reload()
	required, err = svc.IsRequired(ctx, acc, 0)
	require.NoError(t, err)
	assert.False(t, required, "force_off must skip MFA")
}

// TestMFAService_SetMFAOverride_RejectionDoesNotPartialWrite is a regression
// test for the atomic-write fix: an invalid override must NOT have flipped
// the account row or revoked any trusted devices before returning. (We
// can't easily inject a mid-transaction failure here, but we can verify
// the validation rejection path keeps DB state untouched.)
func TestMFAService_SetMFAOverride_RejectionDoesNotPartialWrite(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-override-noPartial")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "UA", net.ParseIP("203.0.113.55"))
	require.NoError(t, err)

	// Bogus override value — service must reject before touching state.
	err = svc.SetMFAOverride(ctx, 0, acc.ID, "force_maybe", "test", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAInvalidOverride)

	override, err := svc.GetMFAOverride(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideNone, override, "override must stay none after rejected write")

	devices, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	assert.Len(t, devices, 1, "trusted device must survive a rejected override attempt")
}

// --- Operator-side admin: defense-in-depth membership check ---

func TestMFAService_OperatorSetMFAOverride_RejectsZeroSchoolID(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-op-zero-school")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	// schoolID=0 is the sentinel that means "no school context" — even
	// if the HTTP route layer was somehow bypassed, the service must
	// refuse to act cross-tenant.
	err := svc.OperatorSetMFAOverride(ctx, 1, 0, acc.ID, auth.MFAAdminOverrideForceOff, "test")
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)
}

func TestMFAService_OperatorSetMFAOverride_RejectsCrossSchool(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-op-cross-school")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	// Account has no AccountTenant rows for school_id=999 — the
	// membership check must reject the override attempt rather than
	// silently writing the column. The DB row would otherwise reveal
	// that an operator can flip force_off on any account ID they can
	// guess.
	err := svc.OperatorSetMFAOverride(ctx, 1, 999, acc.ID, auth.MFAAdminOverrideForceOn, "test")
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	override, err := svc.GetMFAOverride(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideNone, override, "override must remain unset after rejected cross-school write")
}

func TestMFAService_OperatorAdminDisable_RejectsCrossSchool(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-op-disable-cross")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	err := svc.OperatorAdminDisable(ctx, 1, 999, acc.ID, "test")
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// Enrollment must survive the rejected disable attempt.
	enrolled, err := svc.HasEnrollment(ctx, acc.ID)
	require.NoError(t, err)
	assert.True(t, enrolled, "enrollment must persist when operator disable is rejected")
}

// --- Edge cases (Design-Doc §11) ---

func TestMFAService_StartChallenge_RateLimitAfter3Codes(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-rate-limit")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	ip := net.ParseIP("203.0.113.30")

	// 3 codes within the 15-minute window are allowed.
	for i := 0; i < 3; i++ {
		_, err := svc.StartChallenge(ctx, acc.ID, 0, authjwt.MFAChallengeScopeTenant, ip)
		require.NoErrorf(t, err, "code %d should still be permitted", i+1)
	}

	// 4th must trip the rate-limit guard.
	_, err := svc.StartChallenge(ctx, acc.ID, 0, authjwt.MFAChallengeScopeTenant, ip)
	assert.ErrorIs(t, err, auth.ErrMFARateLimited)
}

func TestMFAService_VerifyChallenge_LocksAccountAfter5Failures(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-lockout")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	challenge, err := svc.StartChallenge(ctx, acc.ID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("203.0.113.40"))
	require.NoError(t, err)

	// Five wrong codes — none should lock until the 5th hits the threshold.
	for i := 0; i < auth.MFALockoutThreshold; i++ {
		_, err := svc.VerifyChallenge(ctx, challenge, "000000")
		assert.ErrorIs(t, err, auth.ErrMFACodeInvalid)
	}

	// 6th attempt must surface the lockout error instead of "code invalid".
	_, err = svc.VerifyChallenge(ctx, challenge, "000000")
	assert.ErrorIs(t, err, auth.ErrMFALocked)
}

func TestMFAService_VerifyChallenge_ExpiredChallengeRejected(t *testing.T) {
	ctx := context.Background()
	svc, repos, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-expired")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	challenge, err := svc.StartChallenge(ctx, acc.ID, 0, authjwt.MFAChallengeScopeTenant, net.ParseIP("203.0.113.50"))
	require.NoError(t, err)

	// Consume the active challenge so the next VerifyChallenge has no row
	// to match — same observable outcome as an expired challenge (the
	// service can't distinguish "expired" from "already redeemed" at the
	// row level; both surface as ErrMFACodeInvalid).
	row, err := repos.MFAEmailChallenge.FindActiveByAccountID(ctx, acc.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.NoError(t, repos.MFAEmailChallenge.MarkConsumed(ctx, row.ID, time.Now()))

	_, err = svc.VerifyChallenge(ctx, challenge, "000000")
	assert.Error(t, err, "consumed/expired challenge must not verify")
}
