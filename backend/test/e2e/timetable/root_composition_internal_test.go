package e2e_timetable

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func buildTimetableScenarioModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.TimetableScenarioTestModule, api.ActivePresence, api.ActivePeople, *api.CallerRows, api.ActiveSettings, schoolcalendar.Calendar) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, _, calendar, err := api.NewTimetableScenarioTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	livePeople := api.NewActiveTestPeople(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveCaller := api.NewActiveTestCaller(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	liveSettings := api.NewActiveTestSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	livePresence := api.NewActiveTestPresence(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	return db, module, livePresence, livePeople, liveCaller, liveSettings, calendar
}
