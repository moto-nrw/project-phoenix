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

// A change to an account's second factor commits only with its row in the
// school's authentication ledger (#6989). When the ledger refuses the row,
// the administrator's reset and override, the account's own disable and a
// remembered device roll back and the call fails as retryable.
func TestMFAChangeAuditFailureRollsBackTheChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	svc := newMFATestModule(t, db, withDeliveringMailer(nil)...).MFA
	// The fixture maps the account to the test's school alone, as the
	// account-wide reset requires (#6862).
	target := testpkg.CreateTestAccount(t, db, "mfa-audit-rollback")
	tenantID := testpkg.Tenant(t)
	schoolCtx := testpkg.TenantContext(tenantID)
	require.NoError(t, enrollMFA(ctx, t, db, svc, target.ID))
	const actorID int64 = 9999
	admin := []string{"users:manage"}

	rejectEvent := func(eventType string) func() {
		_, err := db.ExecContext(ctx, `ALTER TABLE audit.auth_events ADD CONSTRAINT reject_mfa_audit_test CHECK (event_type <> ?)`, eventType)
		require.NoError(t, err)
		return func() {
			_, err := db.ExecContext(ctx, `ALTER TABLE audit.auth_events DROP CONSTRAINT reject_mfa_audit_test`)
			require.NoError(t, err)
		}
	}
	enrolled := func() bool {
		ok, err := svc.HasMFAEnrollment(ctx, target.ID)
		require.NoError(t, err)
		return ok
	}
	events := func(eventType string) int64 {
		count, err := db.NewSelect().TableExpr("audit.auth_events").
			Where("account_id = ? AND tenant_id = ? AND event_type = ? AND success", target.ID, tenantID, eventType).Count(ctx)
		require.NoError(t, err)
		return count
	}

	accept := rejectEvent("mfa_admin_override")
	err := svc.SetMFAOverride(schoolCtx, actorID, tenantID, target.ID, identityaccess.MFAAdminOverrideForceOn, "audit outage", admin)
	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable, "an override whose row cannot be written fails")
	override, err := svc.GetTenantMFAOverride(ctx, target.ID, tenantID)
	require.NoError(t, err)
	assert.Equal(t, identityaccess.MFAAdminOverrideNone, override, "the override rolls back with its row")

	err = svc.AdminDisableMFA(schoolCtx, actorID, tenantID, target.ID, "lost mailbox", admin)
	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable, "a reset whose row cannot be written fails")
	assert.True(t, enrolled(), "the reset rolls back with its row")
	accept()

	accept = rejectEvent("mfa_trusted_device_added")
	_, _, err = svc.IssueTrustedDevice(ctx, target.ID, identityaccess.TenantPortalDevice(tenantID), "Mozilla/5.0 (Test)", net.ParseIP("203.0.113.7"))
	require.ErrorIs(t, err, identityaccess.ErrMFAStatusUnavailable, "a device whose row cannot be written is not remembered")
	devices, err := db.NewSelect().TableExpr("auth.mfa_trusted_devices").Where("account_id = ?", target.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, devices)
	accept()

	accept = rejectEvent("mfa_disabled")
	require.ErrorIs(t, svc.DisableMFA(schoolCtx, target.ID), identityaccess.ErrMFAStatusUnavailable,
		"a disable whose row cannot be written fails")
	assert.True(t, enrolled(), "the factor stays with the failed disable")
	accept()

	require.NoError(t, svc.AdminDisableMFA(schoolCtx, actorID, tenantID, target.ID, "lost mailbox", admin))
	assert.False(t, enrolled(), "with a working ledger the reset goes through")
	assert.EqualValues(t, 1, events("mfa_admin_override"))
	assert.EqualValues(t, 1, events("mfa_disabled"))
}
