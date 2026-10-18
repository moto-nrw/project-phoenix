package httpintegration_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/compose/httpadapter"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupActivitiesModule(t *testing.T) (*bun.DB, api.ActivitiesTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return db, module
}

func setupScheduleModule(t *testing.T) (*bun.DB, api.ScheduleTestModule, schedulesAPI.Dateframes) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, calendar, err := api.NewScheduleTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module, calendar
}

func setupTimetableModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.TimetableTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, _, _, err := api.NewTimetableTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}
