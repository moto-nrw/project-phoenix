package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The legacy repair (1.15.205) acts only on positive evidence: a child the care
// plan already booked that day did not come and gets the absence the old
// force-completion path never wrote.
//
// Everyone else is left exactly as they are. A child the current plan does not
// book that weekday is NOT stamped as a non-booking, because a weekday entry
// deleted since leaves no trace anywhere — it is indistinguishable from one
// that never existed, and the marker cannot be undone. A child with no plan at
// all is the same problem in a louder form.
func TestBackfillCompletedAttendanceSplitsByCarePlan(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	// A Monday well in the past: the weekly plan below books its child on the
	// instance's ISO weekday, and nothing about the assertion depends on today.
	date := timezone.NewDate(2025, time.March, 3)
	require.Equal(t, time.Monday, date.Weekday(), "fixture date must be the booked weekday")

	room := testpkg.CreateTestRoom(t, db, "Backfill-Room")
	staff := testpkg.CreateTestStaff(t, db, "Backfill", "Planner")
	booked := testpkg.CreateTestStudent(t, db, "Gebucht", "Montag", "1a")
	unbooked := testpkg.CreateTestStudent(t, db, "Ungebucht", "Montag", "1a")
	planless := testpkg.CreateTestStudent(t, db, "Ohne", "Plan", "1a")

	// booked: Monday plan. unbooked: a plan that covers Wednesday only —
	// the difference between "not booked today" and "no plan on file".
	bookedPlan := testpkg.CreateTestArrivalSchedule(t, db, booked.ID, scheduleModel.WeekdayMonday, staff.ID, "08:00")
	unbookedPlan := testpkg.CreateTestArrivalSchedule(t, db, unbooked.ID, scheduleModel.WeekdayWednesday, staff.ID, "08:00")

	inst := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status:    scheduleModel.InstanceStatusCompleted,
		StartHHMM: "14:00", EndHHMM: "15:00",
	})
	// The old force-completion path stamped completed_at and left the rows
	// alone, which is exactly the "untouched since completion" shape both
	// migrations key on.
	_, err := db.NewRaw(`UPDATE schedule.activity_instances SET completed_at = NOW() + interval '1 hour' WHERE id = ?`, inst.ID).Exec(ctx)
	require.NoError(t, err)

	rows := map[string]*scheduleModel.InstanceStudent{
		"booked":   testpkg.CreateTestInstanceStudent(t, db, inst.ID, booked.ID, scheduleModel.AttendanceStatusExpected),
		"unbooked": testpkg.CreateTestInstanceStudent(t, db, inst.ID, unbooked.ID, scheduleModel.AttendanceStatusExpected),
		"planless": testpkg.CreateTestInstanceStudent(t, db, inst.ID, planless.ID, scheduleModel.AttendanceStatusExpected),
	}

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students",
			rows["booked"].ID, rows["unbooked"].ID, rows["planless"].ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", bookedPlan.ID, unbookedPlan.ID)
		testpkg.CleanupActivityFixtures(t, db, booked.ID, staff.ID, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, db, unbooked.ID, 0, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, db, planless.ID, 0, 0, 0, 0)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	})

	require.NoError(t, backfillCompletedExpectedAttendanceUp(ctx, db))

	assertRow := func(key, wantStatus string, wantMarker bool) {
		t.Helper()
		var status string
		var notScheduled bool
		require.NoError(t, db.NewRaw(
			`SELECT status, not_scheduled FROM schedule.instance_students WHERE id = ?`, rows[key].ID,
		).Scan(ctx, &status, &notScheduled))
		assert.Equal(t, wantStatus, status, "%s: status", key)
		assert.Equal(t, wantMarker, notScheduled, "%s: not_scheduled marker", key)
	}

	assertRow("booked", scheduleModel.AttendanceStatusAbsent, false)
	assertRow("unbooked", scheduleModel.AttendanceStatusExpected, false)
	assertRow("planless", scheduleModel.AttendanceStatusExpected, false)
}

// The plan tables carry no validity interval, so a weekly row written after the
// instance finished says nothing about that day. Judging a historical row by it
// would either invent an absence or stamp a genuine expectation as a
// non-booking — both irreversible. Such a row is left exactly as it is.
func TestBackfillCompletedAttendanceSkipsPlansWrittenAfterCompletion(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	date := timezone.NewDate(2025, time.March, 3)

	room := testpkg.CreateTestRoom(t, db, "Backfill-Late-Plan-Room")
	staff := testpkg.CreateTestStaff(t, db, "Backfill", "LatePlanner")
	// Same shapes as the split test: one booked on the instance's weekday, one
	// booked only on another — but both plans appear AFTER the completion.
	booked := testpkg.CreateTestStudent(t, db, "Spaeter", "Montagsplan", "1a")
	unbooked := testpkg.CreateTestStudent(t, db, "Spaeter", "Mittwochsplan", "1a")

	bookedPlan := testpkg.CreateTestArrivalSchedule(t, db, booked.ID, scheduleModel.WeekdayMonday, staff.ID, "08:00")
	unbookedPlan := testpkg.CreateTestArrivalSchedule(t, db, unbooked.ID, scheduleModel.WeekdayWednesday, staff.ID, "08:00")

	inst := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status:    scheduleModel.InstanceStatusCompleted,
		StartHHMM: "14:00", EndHHMM: "15:00",
	})
	// Completed an hour ago; the attendance rows are untouched since (so the
	// legacy guard passes) but the care plans were written just now.
	_, err := db.NewRaw(`UPDATE schedule.activity_instances SET completed_at = NOW() - interval '1 hour' WHERE id = ?`, inst.ID).Exec(ctx)
	require.NoError(t, err)

	rows := map[string]*scheduleModel.InstanceStudent{
		"booked":   testpkg.CreateTestInstanceStudent(t, db, inst.ID, booked.ID, scheduleModel.AttendanceStatusExpected),
		"unbooked": testpkg.CreateTestInstanceStudent(t, db, inst.ID, unbooked.ID, scheduleModel.AttendanceStatusExpected),
	}
	_, err = db.NewRaw(
		`UPDATE schedule.instance_students SET updated_at = NOW() - interval '2 hours' WHERE id IN (?, ?)`,
		rows["booked"].ID, rows["unbooked"].ID,
	).Exec(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", rows["booked"].ID, rows["unbooked"].ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", bookedPlan.ID, unbookedPlan.ID)
		testpkg.CleanupActivityFixtures(t, db, booked.ID, staff.ID, 0, 0, 0)
		testpkg.CleanupActivityFixtures(t, db, unbooked.ID, 0, 0, 0, 0)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	})

	require.NoError(t, backfillCompletedExpectedAttendanceUp(ctx, db))

	for _, key := range []string{"booked", "unbooked"} {
		var status string
		var notScheduled bool
		require.NoError(t, db.NewRaw(
			`SELECT status, not_scheduled FROM schedule.instance_students WHERE id = ?`, rows[key].ID,
		).Scan(ctx, &status, &notScheduled))
		assert.Equal(t, scheduleModel.AttendanceStatusExpected, status,
			"%s: a plan written after the completion may not decide that day", key)
		assert.False(t, notScheduled, "%s: and may not stamp a non-booking either", key)
	}
}

// A row somebody reset to 'expected' by hand AFTER the instance finished is a
// deliberate decision, not a leftover of the old path. The migration may not
// touch it — it cannot be rolled back.
func TestBackfillCompletedAttendanceSkipsPostCompletionEdits(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	date := timezone.NewDate(2025, time.March, 3)

	room := testpkg.CreateTestRoom(t, db, "Backfill-Reset-Room")
	staff := testpkg.CreateTestStaff(t, db, "Backfill", "Resetter")
	student := testpkg.CreateTestStudent(t, db, "Manuell", "Zurueckgesetzt", "1a")
	plan := testpkg.CreateTestArrivalSchedule(t, db, student.ID, scheduleModel.WeekdayMonday, staff.ID, "08:00")

	inst := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
		Status:    scheduleModel.InstanceStatusCompleted,
		StartHHMM: "14:00", EndHHMM: "15:00",
	})
	// Completed an hour ago, the row edited since.
	_, err := db.NewRaw(`UPDATE schedule.activity_instances SET completed_at = NOW() - interval '1 hour' WHERE id = ?`, inst.ID).Exec(ctx)
	require.NoError(t, err)

	row := testpkg.CreateTestInstanceStudent(t, db, inst.ID, student.ID, scheduleModel.AttendanceStatusExpected)

	t.Cleanup(func() {
		testpkg.CleanupTableRecords(t, db, "schedule.instance_students", row.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.activity_instances", inst.ID)
		testpkg.CleanupTableRecords(t, db, "schedule.student_arrival_schedules", plan.ID)
		testpkg.CleanupActivityFixtures(t, db, student.ID, staff.ID, 0, 0, 0)
		testpkg.CleanupTableRecords(t, db, "facilities.rooms", room.ID)
	})

	require.NoError(t, backfillCompletedExpectedAttendanceUp(ctx, db))

	var status string
	var notScheduled bool
	require.NoError(t, db.NewRaw(
		`SELECT status, not_scheduled FROM schedule.instance_students WHERE id = ?`, row.ID,
	).Scan(ctx, &status, &notScheduled))
	assert.Equal(t, scheduleModel.AttendanceStatusExpected, status,
		"a post-completion reset stays exactly as the human left it")
	assert.False(t, notScheduled, "and must not be relabelled a non-booking")
}
