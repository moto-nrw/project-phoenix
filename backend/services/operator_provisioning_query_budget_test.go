package services

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestOperatorDeviceRowsQueryBudget pins the statement count of the operator
// device listing (#2676, #3253). iot.devices belongs to the Device Fleet
// owner, so the listing is assembled from the dashboard projection's school
// and organization summaries plus one owner read, inside the administrative
// transaction: flat in the number of devices. The listing uses a fixed
// online window, so a settings read would show up here as well.
func TestOperatorDeviceRowsQueryBudget(t *testing.T) {
	t.Parallel()

	// The counter attaches to this database, so the scenario owns its own
	// clone and cannot see statements from a parallel package.
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	provisioning := buildOperatorProvisioning(t, db).OperatorProvisioning
	counter := testpkg.CaptureQueries(t, db)
	// The operator dashboard is a cross-tenant read; production runs it in the
	// administrative transaction the platform-scope request middleware opens,
	// so the listing joins that transaction and only its reads are counted.
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	listDevices := func() []string {
		t.Helper()
		var statements []string
		require.NoError(t, testpkg.WithinAdminContext(t, ctx, db, func(adminCtx context.Context) error {
			counter.Reset()
			devices, err := provisioning.ListAllDevices(adminCtx)
			require.NoError(t, err)
			require.NotEmpty(t, devices)
			statements = counter.Queries()
			return nil
		}))
		return statements
	}

	for index := range 3 {
		testpkg.CreateTestDeviceForTenant(t, db, tenantID, fmt.Sprintf("BUDGET-A-%d", index))
	}

	before := listDevices()
	testpkg.AssertQueryBudget(t, "repositories.operator.device_rows", before)

	for index := range 5 {
		testpkg.CreateTestDeviceForTenant(t, db, tenantID, fmt.Sprintf("BUDGET-B-%d", index))
	}

	after := listDevices()
	testpkg.AssertQueryBudget(t, "repositories.operator.device_rows", after)

	require.Len(t, after, len(before),
		"operator device listing must stay flat in the number of devices")
}
