package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise the public scan capability through the typed root, with real
// Timetable activities and rosters. No HTTP server or fake query source.
func TestYardScanQueryBudgetDistinctActivities(t *testing.T) {
	t.Parallel()
	isolated := testpkg.SetupIsolatedTestDB(t)
	db, module := setupCheckinModule(t)
	require.Same(t, isolated, db)
	yard := testpkg.CreateTestSystemRoom(t, db, "Schulhof", true)
	device := testpkg.CreateTestDevice(t, db, "yard-budget")
	staff := testpkg.CreateTestStaff(t, db, "Yard", "Budget")
	first := testpkg.CreateTestStudent(t, db, "First", "Yard", "1a")
	second := testpkg.CreateTestStudent(t, db, "Second", "Yard", "1a")
	block := testpkg.CreateTestRunningBlock(t, db, yard.ID, "Yard budget first", first.ID, second.ID)
	presence, err := api.NewCheckinTestPresence(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	counter := testpkg.CaptureQueries(t, db)
	scan := func(student *testpkg.Student) []string {
		card := testpkg.CreateTestRFIDCard(t, db, fmt.Sprintf("YB%d", student.ID))
		testpkg.LinkRFIDToStudent(t, db, student.PersonID, card.ID)
		req := testutil.NewAuthenticatedRequest(t, "POST", "/checkin", nil,
			testutil.WithDeviceContext(device), testutil.WithStaffContext(staff.ID, staff.TenantID))
		counter.Reset()
		require.NoError(t, testpkg.WithTenantTx(t, req.Context(), db, testpkg.Tenant(t), func(ctx context.Context, _ testpkg.Tx) error {
			result, err := module.DeviceScan.Scan(ctx, devicescan.ScanCommand{RFIDTag: card.ID, RoomID: &yard.ID})
			if err != nil {
				return err
			}
			require.NotNil(t, result)
			assert.Equal(t, devicescan.ScanActionCheckedIn, result.Action)
			return nil
		}))
		queries := counter.Queries()
		visits, err := presence.ListVisits(testpkg.Ctx(t), studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
		require.NoError(t, err)
		require.Len(t, visits, 1)
		assert.Equal(t, block.ID, visits[0].ActiveGroupID, "the rostered oldest block stays the target")
		return queries
	}
	small := scan(first)
	for i := 1; i < 8; i++ {
		testpkg.CreateTestRunningBlock(t, db, yard.ID, fmt.Sprintf("Yard budget %d", i))
	}
	large := scan(second)
	t.Logf("distinct activities N=1: %d statements; N=8: %d statements", len(small), len(large))
	assert.Equal(t, len(small), len(large), "yard scans must not grow with distinct activities")
	querybudgets.AssertQueryBudget(t, "modules.devicescan.yard_scan.distinct_activities", large)
}
