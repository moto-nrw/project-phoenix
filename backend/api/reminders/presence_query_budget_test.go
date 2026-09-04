package reminders

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	remindersService "github.com/moto-nrw/project-phoenix/services/reminders"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupReminderPresenceRoute(t *testing.T) *Resource {
	t.Helper()
	db, serviceFactory := testutil.SetupAPITest(t)
	require.NoError(t, serviceFactory.Settings.SetValue(
		testpkg.Ctx(t), "reminders.pickup_upcoming_enabled", true, nil, nil,
	))
	return NewResource(serviceFactory.Reminders, serviceFactory.UserContext, db)
}

func TestPresentStudentsInRoomsQueryBudget(t *testing.T) {
	t.Parallel()
	resource := setupReminderPresenceRoute(t)
	db := resource.db
	staff := testpkg.CreateTestStaff(t, db, "Reminder", "Budget")
	add := func(from, to int) {
		for i := from; i < to; i++ {
			room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("Reminder Budget %d", i))
			group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Reminder Budget %d", i))
			activeGroup := testpkg.CreateTestActiveGroup(t, db, group.ID, room.ID)
			testpkg.CreateTestGroupSupervisor(t, db, staff.ID, activeGroup.ID, "primary")
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueriesForContext(t, db)
	ctx := counter.Context(testpkg.Ctx(t))
	run := func() []string {
		counter.Reset()
		_, err := resource.RemindersService.Compute(ctx, remindersService.Scope{StaffID: staff.ID})
		require.NoError(t, err)
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "services.reminders.present_students_in_rooms.reads", large)
}
