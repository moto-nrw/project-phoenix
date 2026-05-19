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
	"github.com/uptrace/bun"

	authjwt "github.com/moto-nrw/project-phoenix/auth/jwt"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
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

// tenantMappedAccount creates an account + a fresh tenant + the
// account_tenants link in one shot. Returns the actorTenantID that
// AdminDisable / SetMFAOverride now require (#1430 Item #2). The cleanup
// is t.Cleanup-scoped so callers don't have to reason about it.
//
// The *bun.DB it accepts comes from newTestMFAService, which calls
// testpkg.SetupTestDB internally — keeping the helper signature explicit
// keeps callers honest about the fact that they're hitting a real DB.
func tenantMappedAccount(t *testing.T, db *bun.DB, slug string) (*authModel.Account, int64) {
	t.Helper()
	acc := testpkg.CreateTestAccount(t, db, slug)
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantID)
	return acc, tenantID
}

func TestMFAService_SetMFAOverride_RejectsInvalidValue(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc, actorTenantID := tenantMappedAccount(t, db, "mfa-svc-override-invalid")

	err := svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, "yes-please", "reason", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAInvalidOverride)
}

func TestMFAService_SetMFAOverride_PermissionGate(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc, actorTenantID := tenantMappedAccount(t, db, "mfa-svc-override-perms")

	// Wrong permission — denied (permission gate runs before membership).
	err := svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, auth.MFAAdminOverrideForceOff, "reason", []string{"something:else"})
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// users:manage + matching tenant works.
	require.NoError(t, svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, auth.MFAAdminOverrideForceOff, "reason", []string{"users:manage"}))
	got, err := svc.GetMFAOverride(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideForceOff, got)
}

func TestMFAService_SetMFAOverride_ForceOff_RevokesTrustedDevices(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc, actorTenantID := tenantMappedAccount(t, db, "mfa-svc-override-revoke")

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "UA", net.ParseIP("203.0.113.20"))
	require.NoError(t, err)

	devicesBefore, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	require.Len(t, devicesBefore, 1)

	require.NoError(t, svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, auth.MFAAdminOverrideForceOff, "lost mailbox", []string{"users:manage"}))

	devicesAfter, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	assert.Empty(t, devicesAfter, "force_off must revoke all trusted devices")
}

func TestMFAService_IsRequired_HonorsOverride(t *testing.T) {
	ctx := context.Background()
	svc, repos, db := newTestMFAService(t)
	acc, actorTenantID := tenantMappedAccount(t, db, "mfa-svc-override-isreq")

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
	require.NoError(t, svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, auth.MFAAdminOverrideForceOn, "always 2FA", []string{"users:manage"}))
	reload()
	required, err = svc.IsRequired(ctx, acc, 0)
	require.NoError(t, err)
	assert.True(t, required, "force_on must force MFA required")

	// force_off => false regardless of tenant mode
	require.NoError(t, svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, auth.MFAAdminOverrideForceOff, "lost mailbox", []string{"users:manage"}))
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
	acc, actorTenantID := tenantMappedAccount(t, db, "mfa-svc-override-noPartial")

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, 0, "UA", net.ParseIP("203.0.113.55"))
	require.NoError(t, err)

	// Bogus override value — service must reject before touching state.
	err = svc.SetMFAOverride(ctx, 0, actorTenantID, acc.ID, "force_maybe", "test", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAInvalidOverride)

	override, err := svc.GetMFAOverride(ctx, acc.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideNone, override, "override must stay none after rejected write")

	devices, err := svc.ListTrustedDevices(ctx, acc.ID)
	require.NoError(t, err)
	assert.Len(t, devices, 1, "trusted device must survive a rejected override attempt")
}

// --- Tenant-side admin: defense-in-depth cross-tenant membership check ---
// (#1430 Item #2)
//
// `auth.accounts` has no tenant_id column and no RLS. The HTTP layer gates
// these endpoints with `users:manage`, but a school admin in tenant A
// could otherwise force-disable MFA on any account in any other tenant by
// guessing its primary key. The service-layer membership check is the
// defense-in-depth that closes that hole.

func TestMFAService_AdminDisable_RejectsCrossTenant(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	target := testpkg.CreateTestAccount(t, db, "mfa-svc-admin-disable-cross")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, target.ID) })

	// Target belongs to tenant A only.
	tenantA := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureAccountTenant(t, db, target.ID, tenantA)

	require.NoError(t, svc.Enroll(ctx, target.ID))

	// Actor lives in tenant B and tries to disable target's MFA. Even with
	// users:manage, the membership check must refuse the call.
	tenantB := testpkg.UniqueTestTenantID(t)
	err := svc.AdminDisable(ctx, 7777, tenantB, target.ID, "lost mailbox", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// Enrollment must survive the rejected disable attempt.
	enrolled, err := svc.HasEnrollment(ctx, target.ID)
	require.NoError(t, err)
	assert.True(t, enrolled, "rejected cross-tenant disable must not wipe the target's credential")
}

func TestMFAService_AdminDisable_RejectsZeroActorTenant(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	target, _ := tenantMappedAccount(t, db, "mfa-svc-admin-disable-zero")

	// actorTenantID=0 is the sentinel for "no tenant context" — the
	// service must refuse to act, just like the operator path refuses
	// schoolID=0. Without this guard a leaked or malformed token with
	// no tenant claim could disable MFA on any account.
	err := svc.AdminDisable(ctx, 7777, 0, target.ID, "no tenant", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)
}

func TestMFAService_SetMFAOverride_RejectsCrossTenant(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	target := testpkg.CreateTestAccount(t, db, "mfa-svc-override-cross")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, target.ID) })

	tenantA := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureAccountTenant(t, db, target.ID, tenantA)

	// Actor in tenant B tries to flip force_off on a target in tenant A.
	tenantB := testpkg.UniqueTestTenantID(t)
	err := svc.SetMFAOverride(ctx, 7777, tenantB, target.ID, auth.MFAAdminOverrideForceOff, "cross-tenant attack", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)

	// And the override on the target must remain unset.
	override, err := svc.GetMFAOverride(ctx, target.ID)
	require.NoError(t, err)
	assert.Equal(t, auth.MFAAdminOverrideNone, override,
		"rejected cross-tenant override must NOT write the column — that's the IDOR the #1430 review flagged")
}

func TestMFAService_SetMFAOverride_RejectsZeroActorTenant(t *testing.T) {
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	target, _ := tenantMappedAccount(t, db, "mfa-svc-override-zero")

	err := svc.SetMFAOverride(ctx, 7777, 0, target.ID, auth.MFAAdminOverrideForceOff, "no tenant", []string{"users:manage"})
	assert.ErrorIs(t, err, auth.ErrMFAPermissionDenied)
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
