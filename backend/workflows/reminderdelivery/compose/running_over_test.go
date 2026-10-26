package compose_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunningOverReadsTheExecutionState runs the running-over reminder (#4765)
// over the real readers. The plan table alone knows only planned and
// cancelled, so the reminder can only see a started or completed block
// through the execution state of Student Presence. The same read keeps a
// started block out of "Aktivität nicht gestartet".
func TestRunningOverReadsTheExecutionState(t *testing.T) {
	t.Parallel()
	day := testpkg.Date(2026, time.September, 4)
	now := day.BerlinMidnight().Add(16*time.Hour + 30*time.Minute)
	db, module, settings := setupRemindersModule(t, func() time.Time { return now })
	ctx := testpkg.Ctx(t)
	require.NoError(t, settings.Settings.SetValue(ctx, "reminders.activity_running_over_enabled", true, nil, nil))
	require.NoError(t, settings.Settings.SetValue(ctx, "reminders.activity_running_over_minutes", 10, nil, nil))
	require.NoError(t, settings.Settings.SetValue(ctx, "reminders.activity_overdue_enabled", true, nil, nil))

	staff := testpkg.CreateTestStaff(t, db, "RunningOver", "Staff")
	activity := testpkg.CreateTestActivityGroup(t, db, "Fußball-AG")

	runningRoom := testpkg.CreateTestRoom(t, db, "Turnhalle RunningOver")
	session := testpkg.CreateTestActiveGroup(t, db, activity.ID, runningRoom.ID)
	running := testpkg.CreateTestActivityInstance(t, db, day, runningRoom.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Fußball-AG",
		ActivityGroupID: &activity.ID, ActiveGroupID: &session.ID,
	})

	completedRoom := testpkg.CreateTestRoom(t, db, "Werkraum RunningOver")
	testpkg.CreateTestActivityInstance(t, db, day, completedRoom.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "15:00", EndHHMM: "16:00", Title: "Werk-AG",
		Status: testpkg.ScheduleInstanceStatusCompleted,
	})

	// A spontaneous block's end is a placeholder: it never runs over.
	spontaneousRoom := testpkg.CreateTestRoom(t, db, "Hof RunningOver")
	spontaneousActivity := testpkg.CreateTestActivityGroup(t, db, "Spontan im Hof")
	spontaneousSession := testpkg.CreateTestActiveGroup(t, db, spontaneousActivity.ID, spontaneousRoom.ID)
	testpkg.CreateTestActivityInstance(t, db, day, spontaneousRoom.ID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Spontan im Hof", IsSpontaneous: true,
		ActivityGroupID: &spontaneousActivity.ID, ActiveGroupID: &spontaneousSession.ID,
	})

	pickedUpLate := testpkg.CreateTestStudent(t, db, "Anna", "Läufer", "3b")
	stillEarly := testpkg.CreateTestStudent(t, db, "Ben", "Läufer", "3b")
	entry := day.BerlinMidnight().Add(15 * time.Hour)
	testpkg.CreateTestVisit(t, db, pickedUpLate.ID, session.ID, entry, nil)
	testpkg.CreateTestVisit(t, db, stillEarly.ID, session.ID, entry, nil)
	testpkg.CreateTestPickupException(t, db, pickedUpLate.ID, day, staff.ID, "16:15", "fixture")
	testpkg.CreateTestPickupException(t, db, stillEarly.ID, day, staff.ID, "17:00", "fixture")

	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context) error {
		single, err := module.Reminders.Compute(ctx, reminder.Scope{IsAdmin: true})
		require.NoError(t, err)
		require.Len(t, single.Reminders, 1, "only the planned, started block runs over; the completed and the spontaneous one do not")
		got := single.Reminders[0]
		assert.Equal(t, reminder.TypeActivityRunningOver, got.Type,
			"a started block is not reported as not started")
		require.NotNil(t, got.ActivityInstanceID)
		assert.Equal(t, strconv.FormatInt(running.ID, 10), *got.ActivityInstanceID)
		assert.Equal(t, "Fußball-AG", got.Title)
		assert.Equal(t, runningRoom.Name, got.Subtitle)
		assert.Equal(t, "16:00", got.DueTime)
		assert.Equal(t, -30, got.MinutesAway)
		require.NotNil(t, got.PickupDueCount)
		assert.Equal(t, 1, *got.PickupDueCount)

		batch, err := module.Reminders.ComputeBatch(ctx, []reminder.BatchScope{{Scope: reminder.Scope{IsAdmin: true, StaffID: staff.ID}}})
		require.NoError(t, err)
		require.Contains(t, batch, staff.ID)
		assert.Equal(t, single.Reminders, batch[staff.ID].Reminders)
		return nil
	}))
}
