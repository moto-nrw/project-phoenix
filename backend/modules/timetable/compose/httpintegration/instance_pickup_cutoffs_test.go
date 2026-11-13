package httpintegration_test

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The range read must keep only requested child/date pairs, not the cartesian
// product of all children and dates, and only automatic partial absences.
func TestInstancePickupCutoffsKeepSelectedPairs(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, module := setupTimetableModule(t)
	ctx := testpkg.Ctx(t)
	day := calendar.NewDate(2027, 4, 19)
	room := testpkg.CreateTestRoom(t, db, "Pickup window")
	staff := testpkg.CreateTestStaff(t, db, "Pickup", "Window")
	a := testpkg.CreateTestStudent(t, db, "Pickup", "First", "1a")
	b := testpkg.CreateTestStudent(t, db, "Pickup", "Last", "1a")
	manual := testpkg.CreateTestStudent(t, db, "Pickup", "Manual", "1a")
	noTime := testpkg.CreateTestStudent(t, db, "Pickup", "No time", "1a")
	unplanned := testpkg.CreateTestStudent(t, db, "Pickup", "Unplanned", "1a")
	first := testpkg.CreateTestActivityInstance(t, db, day, room.ID, testpkg.ActivityInstanceOpts{})
	last := testpkg.CreateTestActivityInstance(t, db, day.AddDays(2), room.ID, testpkg.ActivityInstanceOpts{})
	testpkg.CreateTestInstanceStudent(t, db, first.ID, a.ID, timetable.SlotAttendanceExpected)
	for _, id := range []int64{b.ID, manual.ID, noTime.ID} {
		testpkg.CreateTestInstanceStudent(t, db, last.ID, id, timetable.SlotAttendanceExpected)
	}
	cutoff := calendar.NormalizeWallClock(time.Date(1, 1, 1, 14, 45, 0, 0, time.UTC))
	add := func(id int64, date calendar.Date, automatic, withTime bool) {
		t.Helper()
		row := testpkg.CreateTestPickupException(t, db, id, date, staff.ID, "14:45", "Pickup cutoff")
		var excusedBy *int64
		if withTime && !automatic {
			excusedBy = &staff.ID
		}
		var excusedFrom *time.Time
		if withTime {
			excusedFrom = &cutoff
		}
		_, err := db.NewUpdate().TableExpr("schedule.student_pickup_exceptions").
			Set("excused_auto = ?", automatic).Set("excused_from = ?", excusedFrom).
			Set("excused_created_by = ?", excusedBy).
			Where("id = ? AND tenant_id = ?", row.ID, testpkg.Tenant(t)).Exec(ctx)
		require.NoError(t, err)
	}
	add(a.ID, day, true, true)
	add(b.ID, day.AddDays(2), true, true)
	add(a.ID, day.AddDays(2), true, true) // Selected child, wrong date.
	add(b.ID, day, true, true)            // Selected date, wrong child.
	add(a.ID, day.AddDays(1), true, true) // Date without a selected block.
	add(a.ID, day.AddDays(-1), true, true)
	add(b.ID, day.AddDays(3), true, true)
	add(manual.ID, day.AddDays(2), false, true)
	add(noTime.ID, day.AddDays(2), false, false)
	add(unplanned.ID, day, true, true)

	counter := testpkg.CaptureQueries(t, db)
	rows, err := module.TimetableData.Data.ListScheduledInstanceRows(ctx, []timetable.ScheduledInstance{
		{ID: first.ID, Date: day}, {ID: last.ID, Date: day.AddDays(2)},
	})
	require.NoError(t, err)
	assert.Len(t, counter.Selects("schedule.student_pickup_exceptions"), 1, "one pickup read for the complete window")
	assert.Equal(t, map[calendar.Date]map[int64]time.Time{
		day: {a.ID: cutoff}, day.AddDays(2): {b.ID: cutoff},
	}, rows.Cutoffs)

	empty := testpkg.CreateTestActivityInstance(t, db, day.AddDays(4), room.ID, testpkg.ActivityInstanceOpts{})
	counter.Reset()
	rows, err = module.TimetableData.Data.ListScheduledInstanceRows(ctx, []timetable.ScheduledInstance{{ID: empty.ID, Date: day.AddDays(4)}})
	require.NoError(t, err)
	assert.Empty(t, rows.Cutoffs)
	rows, err = module.TimetableData.Data.ListScheduledInstanceRows(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows.Cutoffs)
	assert.Empty(t, counter.Selects("schedule.student_pickup_exceptions"), "no pickup read without participants")
}
