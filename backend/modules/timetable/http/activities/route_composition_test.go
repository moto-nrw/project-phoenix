package httpadapter_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/api"
	schedulesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func setupActivitiesModule(t *testing.T) (*bun.DB, activityContract.ActivityService, timetable.Capability, api.ActivePeople, *owners.CallerRows) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, activities, users, caller, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return db, activities, module.Timetable, api.ActivePeople{Users: users}, caller
}

func setupScheduleModule(t *testing.T) (*bun.DB, api.ScheduleTestModule, schedulesAPI.Dateframes) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, calendar, err := api.NewScheduleTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	return db, module, calendar
}
