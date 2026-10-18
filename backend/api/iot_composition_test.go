package api

import (
	"testing"
	"time"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupIoTActiveModule(t *testing.T) (*bun.DB, ActiveTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewActiveTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupIoTSettingsModule(t *testing.T) (*bun.DB, settingsCompose.SettingsService) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module.Settings
}

func setupIoTDataModule(t *testing.T) (*bun.DB, IoTDataTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := NewIoTDataTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module
}

func setupIoTTimetableModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, TimetableTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, _, _, err := NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}
