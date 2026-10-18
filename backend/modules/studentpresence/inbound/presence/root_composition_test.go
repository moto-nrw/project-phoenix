package presence_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/inbound/presence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupActiveModule(t *testing.T) (*bun.DB, api.ActiveSettings, api.ActivePresence, api.ActivePeople, api.ActiveEducation, *api.CallerRows, api.ActiveTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewActiveTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	modulePresence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	modulePeople := api.NewActiveTestPeople(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	moduleEducation := api.NewActiveTestEducation(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	moduleCaller := api.NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	moduleSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	return db, moduleSettings, modulePresence, modulePeople, moduleEducation, moduleCaller, module
}

// The root imports this package, so the internal tests reach its settings
// service through this binding.
func init() {
	presence.ComposeTrackingSettings = func(t *testing.T, db *bun.DB) presence.TrackingSettings {
		t.Helper()
		module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
		require.NoError(t, err)
		return module.Settings
	}
}
