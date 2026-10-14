package e2e_timetable

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The typed root's test compositions these suites build: each opens the
// package's test database and composes the module through the root (#4005).

func buildTimetableScenarioModule(t *testing.T, clocks ...func() time.Time) (*bun.DB, api.TimetableScenarioTestModule) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewTimetableScenarioTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
	require.NoError(t, err)
	return db, module
}
