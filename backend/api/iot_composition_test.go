package api

import (
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupIoTActiveModule(t *testing.T) (*bun.DB, DeviceTestModule, ActiveTimetable, activityContract.ActivityService, ActivePresence, ActivePeople) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	_, err := NewActiveTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	moduleTimetable, activityService := NewActiveTestTimetable(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	modulePresence := NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	modulePeople := NewActiveTestPeople(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	moduleDevices := NewActiveTestDevices(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	return db, moduleDevices, moduleTimetable, activityService, modulePresence, modulePeople
}

func setupIoTSettingsModule(t *testing.T) (*bun.DB, settingsCompose.SettingsService) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module.Settings
}

func setupIoTDataModule(t *testing.T) (*bun.DB, IoTDataTestModule, DeviceTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, _, _, _, devices, _, _, err := NewIoTDataTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module, devices
}

func setupIoTTimetableModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, TimetableTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, _, _, err := NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}
