package staffmessaging_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupStaffMessagingModule(t *testing.T) (*bun.DB, api.StaffMessagingTestModule, api.SettingsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, settings, err := api.NewStaffMessagingTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module, settings
}
