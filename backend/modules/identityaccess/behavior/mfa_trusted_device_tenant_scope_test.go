package behavior_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantA)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantB)

	require.NoError(t, enrollMFA(ctx, t, db, svc, acc.ID))

	// Issue the cookie in tenant A.
	cookie, _, err := svc.IssueTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantA), "UA-A", net.ParseIP("203.0.113.30"))
	require.NoError(t, err)
	require.NotEmpty(t, cookie)

	// Same cookie verifies in tenant A (sanity check — the fix didn't break the
	// happy path).
	ok, err := svc.VerifyTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantA), cookie)
	require.NoError(t, err)
	assert.True(t, ok, "tenant A's cookie must verify in tenant A")

	// Cookie MUST NOT verify in tenant B. Pre-fix this would be true (the
	// row had no tenant filter) and an attacker who phished/stole the
	// cookie in one tenant could bypass MFA on any other tenant the same
	// account had access to.
	ok, err = svc.VerifyTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantB), cookie)
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

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	require.NoError(t, enrollMFA(ctx, t, db, svc, acc.ID))

	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantA), "UA-A", net.ParseIP("203.0.113.40"))
	require.NoError(t, err)
	_, _, err = svc.IssueTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantB), "UA-B", net.ParseIP("203.0.113.41"))
	require.NoError(t, err)

	listA, err := svc.ListTrustedDevices(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantA))
	require.NoError(t, err)
	assert.Len(t, listA, 1, "tenant A's list must contain only the tenant-A device")

	listB, err := svc.ListTrustedDevices(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantB))
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

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	require.NoError(t, enrollMFA(ctx, t, db, svc, acc.ID))

	// Trust a device in tenant B.
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantB), "UA", net.ParseIP("203.0.113.50"))
	require.NoError(t, err)

	listB, err := svc.ListTrustedDevices(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantB))
	require.NoError(t, err)
	require.Len(t, listB, 1)
	deviceID := listB[0].ID

	// Attacker authenticated as the same account but in tenant A tries to
	// revoke the tenant-B device by passing its row id. Must be refused
	// because the row doesn't belong to (acc, tenantA).
	err = svc.RevokeTrustedDevice(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantA), deviceID)
	assert.Error(t, err,
		"a token-A revoke against a tenant-B device must NOT succeed")

	// And the device is still active in tenant B.
	stillThere, err := svc.ListTrustedDevices(ctx, acc.ID, identityaccess.TenantPortalDevice(tenantB))
	require.NoError(t, err)
	assert.Len(t, stillThere, 1, "rejected cross-tenant revoke must leave the row untouched")
}

// TestMFAService_TrustedDevice_CountsOnlyInItsPortal holds the binding of
// #6955: a remembered device answers only in the portal that remembered it,
// at its school or Träger. The tenant portal and moto schule of one school,
// the caterer portal and the Träger-Portal, and two Träger never share one,
// neither for the sign-in, nor for the list, nor for a revoke.
func TestMFAService_TrustedDevice_CountsOnlyInItsPortal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc := testpkg.CreateTestAccount(t, db, "mfa-svc-portal-scope")
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	testpkg.EnsureAccountTenant(t, db, acc.ID, school)
	traegerA := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger A", Slug: "geraete-traeger-a-" + acc.Email})
	traegerB := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger B", Slug: "geraete-traeger-b-" + acc.Email})
	require.NoError(t, enrollMFA(ctx, t, db, svc, acc.ID))

	scopes := map[string]identityaccess.TrustedDeviceScope{
		"tenant":    identityaccess.TenantPortalDevice(school),
		"school":    identityaccess.SchoolPortalDevice(school),
		"caterer":   identityaccess.CatererPortalDevice(),
		"traeger-a": identityaccess.TraegerPortalDevice(traegerA.ID),
		"traeger-b": identityaccess.TraegerPortalDevice(traegerB.ID),
	}
	cookies := map[string]string{}
	for name, scope := range scopes {
		cookie, _, err := svc.IssueTrustedDevice(ctx, acc.ID, scope, "UA-"+name, net.ParseIP("203.0.113.60"))
		require.NoError(t, err, name)
		require.NotEmpty(t, cookie, name)
		cookies[name] = cookie
	}
	for issued, cookie := range cookies {
		for checked, scope := range scopes {
			ok, err := svc.VerifyTrustedDevice(ctx, acc.ID, scope, cookie)
			require.NoError(t, err)
			assert.Equal(t, issued == checked, ok, "a device remembered in %s must answer only there, not in %s", issued, checked)
		}
	}
	for name, scope := range scopes {
		listed, err := svc.ListTrustedDevices(ctx, acc.ID, scope)
		require.NoError(t, err)
		require.Len(t, listed, 1, "the %s list shows its own device only", name)
		assert.Equal(t, "UA-"+name, *listed[0].UserAgent)
	}

	tenantDevices, err := svc.ListTrustedDevices(ctx, acc.ID, scopes["tenant"])
	require.NoError(t, err)
	assert.Error(t, svc.RevokeTrustedDevice(ctx, acc.ID, scopes["school"], tenantDevices[0].ID),
		"moto schule must not revoke a device of the tenant portal")
	traegerDevices, err := svc.ListTrustedDevices(ctx, acc.ID, scopes["traeger-a"])
	require.NoError(t, err)
	assert.Error(t, svc.RevokeTrustedDevice(ctx, acc.ID, scopes["traeger-b"], traegerDevices[0].ID),
		"one Träger must not revoke a device of another")
	assert.Error(t, svc.RevokeTrustedDevice(ctx, acc.ID, scopes["caterer"], traegerDevices[0].ID),
		"the caterer portal must not revoke a device of the Träger-Portal")
	require.NoError(t, svc.RevokeTrustedDevice(ctx, acc.ID, scopes["traeger-a"], traegerDevices[0].ID))
	ok, err := svc.VerifyTrustedDevice(ctx, acc.ID, scopes["traeger-a"], cookies["traeger-a"])
	require.NoError(t, err)
	assert.False(t, ok, "a revoked device no longer skips the code")

	_, _, err = svc.IssueTrustedDevice(ctx, acc.ID, identityaccess.TrustedDeviceScope{Portal: "parent", TenantID: school}, "UA", nil)
	assert.Error(t, err, "a portal without a second factor remembers no device")
	_, _, err = svc.IssueTrustedDevice(ctx, acc.ID, identityaccess.TraegerPortalDevice(0), "UA", nil)
	assert.Error(t, err, "a Träger-Portal device needs its Träger")
}
