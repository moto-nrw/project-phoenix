package compose_test

import (
	"context"
	"testing"

	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestActiveTerminalCountsCountOnlyLiveActivePhysicalDevices pins the billing
// rule of #2791: a live, non-virtual device in status active counts. An
// archived (transferred) device, the virtual web device and a device in any
// other status do not. Whether the device was online recently is irrelevant.
func TestActiveTerminalCountsCountOnlyLiveActivePhysicalDevices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	testpkg.CreateTestDevice(t, db, "billing-active")
	tablet := testpkg.CreateTestDevice(t, db, "billing-tablet")
	_, err := db.ExecContext(ctx, "UPDATE iot.devices SET device_type = 'tablet', last_seen = NULL WHERE id = ?", tablet.ID)
	require.NoError(t, err)
	for _, change := range []string{
		"UPDATE iot.devices SET status = 'inactive' WHERE id = ?",
		"UPDATE iot.devices SET status = 'maintenance' WHERE id = ?",
		"UPDATE iot.devices SET status = 'offline' WHERE id = ?",
		"UPDATE iot.devices SET device_type = 'virtual' WHERE id = ?",
		"UPDATE iot.devices SET archived_at = NOW() WHERE id = ?",
	} {
		device := testpkg.CreateTestDevice(t, db, "billing-excluded")
		_, err := db.ExecContext(ctx, change, device.ID)
		require.NoError(t, err)
	}

	counts := devicefleetCompose.NewActiveTerminalCounts()
	require.NoError(t, testpkg.WithinAdminContext(t, context.Background(), db, func(adminCtx context.Context) error {
		byTenant, err := counts.CountActiveTerminalsByTenant(adminCtx)
		require.NoError(t, err)
		require.Equal(t, 2, byTenant[tenantID])
		return nil
	}))
}

// TestActiveTerminalCountsRefuseATenantTransaction pins that the count fails
// instead of silently counting one school outside the administrative
// transaction.
func TestActiveTerminalCountsRefuseATenantTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	counts := devicefleetCompose.NewActiveTerminalCounts()

	_, err := counts.CountActiveTerminalsByTenant(testpkg.Ctx(t))
	require.ErrorContains(t, err, "transaction is required")

	require.NoError(t, testpkg.WithinTenantContext(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(tenantCtx context.Context) error {
		_, err := counts.CountActiveTerminalsByTenant(tenantCtx)
		require.ErrorContains(t, err, "administrative transaction")
		return nil
	}))
}
