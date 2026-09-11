package platform_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestOperatorDeviceRowsQueryBudget pins the statement count of the operator
// device listing. iot.devices belongs to the Device Fleet owner (#2676), so
// the listing is assembled from the owner query plus the school and
// organization summaries instead of one cross-schema join: three statements,
// flat in the number of devices.
func TestOperatorDeviceRowsQueryBudget(t *testing.T) {
	t.Parallel()

	// The counter attaches to this database, so the scenario owns its own
	// clone and cannot see statements from a parallel package.
	db := testpkg.SetupIsolatedTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	factory, err := repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
	require.NoError(t, err)
	// The operator dashboard is a cross-tenant read; production runs it inside
	// the admin transaction, so the context carries the runtime but no tenant.
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	for index := range 3 {
		testpkg.CreateTestDeviceForTenant(t, db, tenantID, fmt.Sprintf("BUDGET-A-%d", index))
	}

	counter := testpkg.CaptureQueries(t, db)
	rows, err := factory.OperatorSummaries.ListDeviceRows(ctx, platformModels.OperatorDeviceFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	before := counter.Queries()
	testpkg.AssertQueryBudget(t, "repositories.operator.device_rows", before)

	for index := range 5 {
		testpkg.CreateTestDeviceForTenant(t, db, tenantID, fmt.Sprintf("BUDGET-B-%d", index))
	}

	counter.Reset()
	rows, err = factory.OperatorSummaries.ListDeviceRows(ctx, platformModels.OperatorDeviceFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	after := counter.Queries()
	testpkg.AssertQueryBudget(t, "repositories.operator.device_rows", after)

	require.Len(t, after, len(before),
		"operator device listing must stay flat in the number of devices")
}
