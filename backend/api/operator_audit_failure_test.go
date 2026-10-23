package api

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperatorAuditFailureRollsBackRotation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := buildOperatorProvisioning(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	_, schoolID := provisionTestSchool(t, db, service, operatorID, "audit-rotation")
	device, err := service.CreateDevice(ctx, schoolID, "AUDIT-ROTATION", "terminal", nil, nil, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	var before string
	require.NoError(t, db.NewRaw(`SELECT api_key_hash FROM iot.devices WHERE id = ?`, device.ID).Scan(ctx, &before))

	_, err = db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
	CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
	FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)

	rotated, err := service.SetDeviceAPIKey(ctx, device.ID, nil, operatorID, provisioningTestClientIP)
	require.Error(t, err)
	assert.Nil(t, rotated, "a failed audit must not return a new credential")
	var after string
	require.NoError(t, db.NewRaw(`SELECT api_key_hash FROM iot.devices WHERE id = ?`, device.ID).Scan(ctx, &after))
	assert.True(t, before == after, "audit failure must roll back the credential change")
}

func TestOperatorAuditFailureRollsBackMFAOverride(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service, identity := buildOperatorProvisioningWithIdentity(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	_, schoolID := provisionTestSchool(t, db, service, operatorID, "audit-mfa")
	account := testpkg.CreateTestAccount(t, db, "audit-mfa")
	testpkg.EnsureAccountTenant(t, db, account.ID, schoolID)
	require.NoError(t, identity.OperatorSetMFAOverride(ctx, operatorID, schoolID, account.ID, identityaccess.MFAAdminOverrideForceOn, "test transition"))
	require.NoError(t, identity.EnrollMFA(ctx, account.ID))
	require.NoError(t, identity.OperatorSetGlobalMFAOverride(ctx, operatorID, account.ID, identityaccess.MFAAdminOverrideForceOn, "test transition"))
	_, err := db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
 CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
 FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)
	t.Run("reset", func(t *testing.T) {
		require.Error(t, identity.OperatorDisableMFA(ctx, operatorID, schoolID, account.ID, "test transition"))
		enrolled, err := identity.HasMFAEnrollment(ctx, account.ID)
		require.NoError(t, err)
		assert.True(t, enrolled, "audit failure must preserve enrollment")
	})

	for _, override := range []string{identityaccess.MFAAdminOverrideForceOff, identityaccess.MFAAdminOverrideNone} {
		t.Run("school-"+override, func(t *testing.T) {
			require.Error(t, identity.OperatorSetMFAOverride(ctx, operatorID, schoolID, account.ID, override, "test transition"))
			state, err := identity.GetTenantMFAOverride(ctx, account.ID, schoolID)
			require.NoError(t, err)
			assert.Equal(t, identityaccess.MFAAdminOverrideForceOn, state)
		})
		t.Run("global-"+override, func(t *testing.T) {
			require.Error(t, identity.OperatorSetGlobalMFAOverride(ctx, operatorID, account.ID, override, "test transition"))
			state, err := identity.GetGlobalMFAOverride(ctx, account.ID)
			require.NoError(t, err)
			assert.Equal(t, identityaccess.MFAAdminOverrideForceOn, state)
		})
	}
}

func TestOperatorAuditFailureRollsBackDeletesAndTransfer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	service := buildOperatorProvisioning(t, db)
	ctx := provisioningContext(t, db)
	operatorID := testpkg.CreateTestOperator(t, db).ID
	organizationID, schoolID := provisionTestSchool(t, db, service, operatorID, "audit-deletes")
	target, err := service.CreateSchool(ctx, &organizationtenancy.CreateSchool{
		OrganizationID: organizationID, Name: "Audit Target", Slug: "audit-target", Subdomain: "audit-target", Active: true,
	}, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	organization, err := service.CreateOrganization(ctx, &organizationtenancy.CreateOrganization{Name: "Empty Audit Organization", Slug: "empty-audit-org", Active: true}, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	device, err := service.CreateDevice(ctx, schoolID, "AUDIT-DEVICE", "terminal", nil, nil, operatorID, provisioningTestClientIP)
	require.NoError(t, err)
	person, account := testpkg.CreateTestPersonWithAccount(t, db, "Audit", "Fixture")
	token := testpkg.CreateTestTokenForTenant(t, db, schoolID, account.ID)
	_, err = db.ExecContext(ctx, `CREATE FUNCTION platform.reject_operator_audit() RETURNS trigger LANGUAGE plpgsql AS $$
	BEGIN RAISE EXCEPTION 'injected audit failure'; END $$;
	CREATE TRIGGER reject_operator_audit BEFORE INSERT ON platform.operator_audit_log
	FOR EACH ROW EXECUTE FUNCTION platform.reject_operator_audit()`)
	require.NoError(t, err)
	for name, action := range map[string]func() error{
		"transfer": func() error {
			_, err := service.TransferDevice(ctx, device.ID, target.ID, operatorID, provisioningTestClientIP)
			return err
		},
		"device": func() error { return service.DeleteDevice(ctx, device.ID, operatorID, provisioningTestClientIP) },
		"school": func() error { return service.SoftDeleteSchool(ctx, schoolID, operatorID, provisioningTestClientIP) },
		"organization": func() error {
			return service.SoftDeleteOrganization(ctx, organization.ID, operatorID, provisioningTestClientIP)
		},
		"person": func() error { return service.SoftDeletePerson(ctx, person.ID, operatorID, provisioningTestClientIP) },
	} {
		t.Run(name, func(t *testing.T) {
			var unavailable *organizationtenancy.OperatorAuditUnavailableError
			require.ErrorAs(t, action(), &unavailable)
		})
	}
	for _, assertion := range []struct {
		query string
		ids   []any
	}{
		{`SELECT count(*) FROM iot.devices WHERE id = ? AND archived_at IS NULL AND api_key_hash IS NOT NULL AND transferred_to_device_id IS NULL`, []any{device.ID}},
		{`SELECT count(*) FROM platform.schools WHERE id = ? AND deleted_at IS NULL`, []any{schoolID}},
		{`SELECT count(*) FROM platform.organizations WHERE id = ? AND deleted_at IS NULL`, []any{organization.ID}},
		{`SELECT count(*) FROM users.persons WHERE id = ? AND deleted_at IS NULL AND account_id = ?`, []any{person.ID, account.ID}},
		{`SELECT count(*) FROM auth.tokens WHERE id = ?`, []any{token.ID}},
	} {
		var count int
		require.NoError(t, db.NewRaw(assertion.query, assertion.ids...).Scan(ctx, &count))
		assert.Equal(t, 1, count, "audit failure must preserve each affected row")
	}
	var targets int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM iot.devices WHERE tenant_id = ? AND device_id = ?`, target.ID, device.DeviceID).Scan(ctx, &targets))
	assert.Zero(t, targets, "a failed transfer must not create a device at the other school")
}
