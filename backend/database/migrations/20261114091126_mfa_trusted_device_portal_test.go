package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestTrustedDevicePortalKeepsEachPortalItsShape checks the binding #6955
// adds: a device of the tenant portal or moto schule names its school, one
// of the caterer names neither school nor Träger, one of the Träger-Portal
// names its Träger and no school, and no other portal remembers a device.
func TestTrustedDevicePortalKeepsEachPortalItsShape(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "trusted-device-portal")
	school := testpkg.Tenant(t)
	traeger := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Gerätebindung", Slug: fmt.Sprintf("traeger-geraete-%d", account.ID),
	})
	insert := func(label, portal string, tenantID, organizationID any) error {
		_, err := db.ExecContext(ctx, `INSERT INTO auth.mfa_trusted_devices
			(account_id, portal, tenant_id, organization_id, token_hash, expires_at)
			VALUES (?, ?, ?, ?, ?, NOW() + INTERVAL '1 day')`,
			account.ID, portal, tenantID, organizationID, "portal-shape-"+label)
		return err
	}
	for _, valid := range []struct {
		label, portal   string
		tenantID, orgID any
	}{
		{"tenant", "tenant", school, nil},
		{"school", "school", school, nil},
		{"caterer", "caterer", nil, nil},
		{"org", "org", nil, traeger.ID},
	} {
		require.NoError(t, insert(valid.label, valid.portal, valid.tenantID, valid.orgID), valid.label)
	}
	for _, invalid := range []struct {
		label, portal   string
		tenantID, orgID any
	}{
		{"tenant-without-school", "tenant", nil, nil},
		{"school-with-traeger", "school", school, traeger.ID},
		{"caterer-with-school", "caterer", school, nil},
		{"org-without-traeger", "org", nil, nil},
		{"org-with-school", "org", school, traeger.ID},
		{"parent", "parent", nil, nil},
		{"platform", "platform", nil, nil},
	} {
		require.Error(t, insert(invalid.label, invalid.portal, invalid.tenantID, invalid.orgID), invalid.label)
	}
	_, err := db.ExecContext(ctx, `INSERT INTO auth.mfa_trusted_devices
		(account_id, tenant_id, token_hash, expires_at) VALUES (?, ?, 'portal-shape-none', NOW() + INTERVAL '1 day')`, account.ID, school)
	require.Error(t, err, "a device without a portal counts nowhere and is refused")
}
