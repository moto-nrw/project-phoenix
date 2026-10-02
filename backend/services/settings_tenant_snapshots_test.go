package services

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Worker's minute snapshot preloads the polling settings of every school
// through settingsCompose.NewTenantSnapshots (SettingsTestModule.TenantSnapshots) over the retained settings
// service (#2746): one config.setting_values read for all schools, and a
// bound snapshot serves the school's reads without another.
func TestSchedulerMinuteSnapshotReadsEverySchoolOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)
	module, err := NewSettingsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	snapshots, err := module.TenantSnapshots()
	require.NoError(t, err)
	const sessionEndTime = "operations.session_end_time"

	counter := testpkg.CaptureQueries(t, db)
	resolved, err := snapshots(context.Background(), []int64{tenantA, tenantB}, []string{sessionEndTime, "gdpr.data_cleanup_enabled"})
	require.NoError(t, err)
	require.Contains(t, resolved, tenantA)
	require.Contains(t, resolved, tenantB)

	ctx := resolved[tenantA].Bind(testpkg.ContextForTenant(context.Background(), tenantA))
	endTime, err := module.Settings.ResolveString(ctx, sessionEndTime)
	require.NoError(t, err)
	assert.NotEmpty(t, endTime)

	testpkg.AssertQueryBudget(t, "services.scheduler.minute_snapshot.setting_values",
		counter.Selects("config.setting_values"))
}
