package httpintegration_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDeletingSupervisorBatchesRosterStaffLocks(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.April, 20)
	s := makeScenario(t, activityContract.WeekdayMonday, day)
	defer s.runCleanup(t)
	_, activities, _, _, err := api.NewActivitiesTestModule(testpkg.NewDatabase(t, s.db))
	require.NoError(t, err)
	queries := testpkg.CaptureQueriesForContext(t, s.db)
	var statementCount int
	for _, size := range []int{2, 6} {
		group := testpkg.CreateTestActivityGroupForTenant(t, s.db, s.tenantID, fmt.Sprintf("Roster of %d", size))
		for i := range size {
			staff := testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Roster", fmt.Sprintf("Member %d", i))
			addPlannedSupervisor(t, s, group.ID, staff.ID, day)
		}
		roster, err := activities.GetGroupSupervisors(s.ctx, group.ID)
		require.NoError(t, err)
		require.Len(t, roster, size)
		queries.Reset()
		require.NoError(t, activities.DeleteSupervisor(queries.Context(s.ctx), roster[len(roster)-1].ID))
		locks := queries.Matching(func(sql string) bool {
			return strings.Contains(sql, "staff_school_memberships") && strings.Contains(sql, "for no key update")
		})
		require.Len(t, locks, 1, "the entire roster must use one staff locking query")
		if statementCount == 0 {
			statementCount = queries.Total()
		} else {
			require.Equal(t, statementCount, queries.Total(), "removing one supervisor must not add queries per roster member")
		}
	}
}
