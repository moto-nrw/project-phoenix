package sessions_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupActiveModule(t *testing.T) (*bun.DB, api.ActiveDeviceScan, api.ActiveSessionEnd) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, err := api.NewActiveTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	moduleSessionEnd := api.NewActiveTestSessionEnd(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	moduleScan := api.NewActiveTestDeviceScan(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	return db, moduleScan, moduleSessionEnd
}
