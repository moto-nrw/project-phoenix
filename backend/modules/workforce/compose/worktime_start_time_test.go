package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both writers of a TIME start_time column bind it through the same adapter
// helper. A clock parsed without a date lands in year 0, which PostgreSQL
// rejects (SQLSTATE 22008, #5675); each writer must store the wall clock
// unchanged and leave an empty start time unset.
func TestWorkTimeStartTimesRoundTripThroughTheDatabase(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Start", "Clock")
	capability := buildWorkforce(t, db)

	t.Run("work-time model entries", func(t *testing.T) {
		created, err := capability.CreateWorkTimeModel(ctx, workforce.CreateWorkTimeModel{
			WorkTimeModelFields: workforce.WorkTimeModelFields{
				Name:               "Clock roundtrip",
				RotationLength:     1,
				RotationAnchorDate: "2026-10-26",
				Entries: []workforce.WorkTimeModelEntry{
					{WeekIndex: 0, DayOfWeek: dayMonday, TargetMinutes: 360, StartTime: "13:45:00"},
					{WeekIndex: 0, DayOfWeek: dayTuesday, TargetMinutes: 240},
				},
			},
		})
		require.NoError(t, err)
		t.Cleanup(func() { _ = capability.DeleteWorkTimeModel(ctx, created.ID) })

		found, err := capability.FindWorkTimeModel(ctx, created.ID)
		require.NoError(t, err)
		starts := map[int]string{}
		for _, entry := range found.Entries {
			starts[entry.DayOfWeek] = entry.StartTime
		}
		assert.Equal(t, map[int]string{dayMonday: "13:45:00", dayTuesday: ""}, starts)
	})

	t.Run("staff work schedule", func(t *testing.T) {
		require.NoError(t, capability.ReplaceStaffSchedule(ctx, workforce.ReplaceStaffSchedule{
			StaffID: staff.ID,
			Entries: []workforce.StaffWorkScheduleEntry{
				{WeekIndex: 0, RotationLength: 1, DayOfWeek: dayMonday, TargetMinutes: 360, StartTime: "13:45:00"},
				{WeekIndex: 0, RotationLength: 1, DayOfWeek: dayTuesday, TargetMinutes: 240},
			},
		}))

		current, err := capability.CurrentStaffSchedule(ctx, staff.ID)
		require.NoError(t, err)
		starts := map[int]string{}
		for _, entry := range current {
			starts[entry.DayOfWeek] = entry.StartTime
		}
		assert.Equal(t, map[int]string{dayMonday: "13:45:00", dayTuesday: ""}, starts)
	})
}
