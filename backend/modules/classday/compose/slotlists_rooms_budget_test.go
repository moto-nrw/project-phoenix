package compose_test

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildList_RoomQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	small, large := measureSlotRoomQueries(t, false)
	assert.Len(t, large, len(small), "distinct rooms must not add statements")
	querybudgets.AssertQueryBudget(t, "modules.classday.slot_list.build_rooms", small)
	querybudgets.AssertQueryBudget(t, "modules.classday.slot_list.build_rooms", large)
}

func TestListOptions_RoomQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	small, large := measureSlotRoomQueries(t, true)
	assert.Len(t, large, len(small), "distinct rooms must not add statements")
	querybudgets.AssertQueryBudget(t, "modules.classday.slot_list.options_rooms", small)
	querybudgets.AssertQueryBudget(t, "modules.classday.slot_list.options_rooms", large)
}

// Grow the selectable room set, not just a roster sharing one room. Each
// measurement owns its database and exercises the tenant transaction binding.
func measureSlotRoomQueries(t *testing.T, options bool) ([]string, []string) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	f := buildMensaFixtureOn(t, db)
	ctx := testpkg.Ctx(t)
	counter := testpkg.CaptureQueries(t, db)
	measure := func(wantSlots int) []string {
		counter.Reset()
		var slots []classday.Slot
		if options {
			result, err := f.svc.ListOptions(ctx, classday.Date(listDate.String()))
			require.NoError(t, err)
			slots = result.Slots
		} else {
			result, err := f.svc.BuildList(ctx, classday.Params{
				Date: classday.Date(listDate.String()), Target: classday.TargetSlots, Source: classday.SourceReconciliation,
			})
			require.NoError(t, err)
			require.Len(t, result.Rows, 3)
			slots = result.Slots
		}
		require.Len(t, slots, wantSlots)
		for _, slot := range slots {
			if slot.InstanceID == f.instanceID {
				require.NotEmpty(t, slot.RoomName)
			} else {
				assert.Equal(t, slot.Title, slot.RoomName, "batch results must map by room ID, not result order")
			}
		}
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
		t.Logf("options=%t slots=%d total=%d room_selects=%d", options, wantSlots, counter.Total(), len(counter.Selects("facilities.rooms")))
		require.Len(t, counter.Selects("facilities.rooms"), 1)
		return counter.Queries()
	}
	small := measure(1)
	for i := 0; i < 7; i++ {
		room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SL-Budget-Room-%d", i))
		inst := &testpkg.ActivityInstance{
			Date: testpkg.ScheduleDate(listDate), Title: room.Name, RoomID: room.ID,
			StartTime: atOn(listDate, 11, 30), EndTime: atOn(listDate, 13, 0), Status: testpkg.ScheduleInstanceStatusPlanned,
		}
		inst.TenantID = testpkg.Tenant(t)
		testpkg.InsertActivityInstanceRow(t, ctx, db, inst)
	}
	return small, measure(8)
}
