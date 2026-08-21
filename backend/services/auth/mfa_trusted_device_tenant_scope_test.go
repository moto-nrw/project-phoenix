package auth_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestMFAService_VerifyTrustedDevice_RejectsCrossTenantCookie exercises the
// security boundary added in #1430 review item #9. Trust must be per-
// (account, tenant): a cookie issued while the user was logged into
// tenant A MUST NOT bypass MFA when the same user later logs into
// tenant B. Pre-fix the auth.mfa_trusted_devices table was scoped by
// account_id only, so the (account, hash) lookup returned the tenant-A
// row regardless of which tenant the verify came from.
func TestMFAService_VerifyTrustedDevice_RejectsCrossTenantCookie(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-cross-tenant-cookie")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantA)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantB)

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	// Issue the cookie in tenant A.
	cookie, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantA, "UA-A", net.ParseIP("203.0.113.30"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	// Same cookie verifies in tenant A (sanity check — the fix didn't break the
	// happy path).
	ok, err := svc.VerifyTrustedDevice(ctx, acc.ID, tenantA, cookie)
	require.NoError(t, err)
	assert.True(t, ok, "tenant A's cookie must verify in tenant A")

	// Cookie MUST NOT verify in tenant B. Pre-fix this would be true (the
	// row had no tenant filter) and an attacker who phished/stole the
	// cookie in one tenant could bypass MFA on any other tenant the same
	// account had access to.
	ok, err = svc.VerifyTrustedDevice(ctx, acc.ID, tenantB, cookie)
	require.NoError(t, err)
	assert.False(t, ok,
		"trusted-device cookie issued for tenant A must NOT bypass MFA on tenant B (#1430 review item #9)")
}

// TestMFAService_ListTrustedDevices_ScopedToTenant proves the settings page
// only shows the devices trusted in the current tenant. Without per-tenant
// scoping a user logged into tenant A would see (and could revoke) devices
// they trusted in tenant B.
func TestMFAService_ListTrustedDevices_ScopedToTenant(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-list-scoped")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantA, "UA-A", net.ParseIP("203.0.113.40"))
	require.NoError(t, err)
	_, _, err = svc.IssueTrustedDevice(ctx, acc.ID, tenantB, "UA-B", net.ParseIP("203.0.113.41"))
	require.NoError(t, err)

	listA, err := svc.ListTrustedDevices(ctx, acc.ID, tenantA)
	require.NoError(t, err)
	assert.Len(t, listA, 1, "tenant A's list must contain only the tenant-A device")

	listB, err := svc.ListTrustedDevices(ctx, acc.ID, tenantB)
	require.NoError(t, err)
	assert.Len(t, listB, 1, "tenant B's list must contain only the tenant-B device")

	assert.NotEqual(t, listA[0].ID, listB[0].ID, "the two lists must surface different rows")
}

// TestMFAService_RevokeTrustedDevice_RejectsCrossTenantRevoke proves an
// attacker holding an access token for (account A, tenant T1) cannot use
// it to revoke a device that account A trusted in tenant T2 — the IDOR
// guard rejects the cross-tenant id-guess.
func TestMFAService_RevokeTrustedDevice_RejectsCrossTenantRevoke(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-revoke-scoped")
	t.Cleanup(func() { testpkg.CleanupAccount(t, db, acc.ID) })

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	require.NoError(t, svc.Enroll(ctx, acc.ID))

	// Trust a device in tenant B.
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantB, "UA", net.ParseIP("203.0.113.50"))
	require.NoError(t, err)

	listB, err := svc.ListTrustedDevices(ctx, acc.ID, tenantB)
	require.NoError(t, err)
	require.Len(t, listB, 1)
	deviceID := listB[0].ID

	// Attacker authenticated as the same account but in tenant A tries to
	// revoke the tenant-B device by passing its row id. Must be refused
	// because the row doesn't belong to (acc, tenantA).
	err = svc.RevokeTrustedDevice(ctx, acc.ID, tenantA, deviceID)
	assert.Error(t, err,
		"a token-A revoke against a tenant-B device must NOT succeed")

	// And the device is still active in tenant B.
	stillThere, err := svc.ListTrustedDevices(ctx, acc.ID, tenantB)
	require.NoError(t, err)
	assert.Len(t, stillThere, 1, "rejected cross-tenant revoke must leave the row untouched")
}
