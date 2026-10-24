package staffclock_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupWorkSessionModule(t *testing.T) (*bun.DB, devicescan.StaffClock) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, staffClock, err := api.NewWorkSessionTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, staffClock
}
