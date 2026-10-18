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

func buildTimetableScenarioModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.TimetableScenarioTestModule, api.ActiveTestModule, schoolcalendar.Calendar) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, live, calendar, err := api.NewTimetableScenarioTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module, live, calendar
}
