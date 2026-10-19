package compose_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupCalendarModule(t *testing.T) (*bun.DB, api.CalendarTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewCalendarTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupRemindersModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.RemindersTestModule, owners.SettingsTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, settings, err := api.NewRemindersTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module, settings
}
