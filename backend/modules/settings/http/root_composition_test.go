package settingshttp_test

import (
	"testing"

	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func composeSettingsModule(t *testing.T) (*bun.DB, api.SettingsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}
