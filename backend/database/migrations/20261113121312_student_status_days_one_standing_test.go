package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOneStandingStatusDayKeepsTheLatestReport pins the cleanup of
// 20261113121312 (#6958): a day with several standing statuses keeps the
// latest report, the others are cleared when it came in, an end-of-day
// archive among them stops standing, and other days stay untouched.
func TestOneStandingStatusDayKeepsTheLatestReport(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, studentStatusDaysOneStandingDownSQL)
	require.NoError(t, err)

	student := testpkg.CreateTestStudent(t, db, "Ida", "Doppelt", "2a")
	tenantID := student.TenantID
	morning := time.Date(2026, 3, 2, 7, 0, 0, 0, time.UTC)
	insert := func(day, status, source string, reportedAt time.Time, clearedAt *time.Time) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, cleared_at, source)
			VALUES (?, ?, ?::date, ?, ?, ?, ?) RETURNING id`,
			tenantID, student.ID, day, status, reportedAt, clearedAt, source).Scan(ctx, &id))
		return id
	}
	archivedAt := morning.Add(10 * time.Hour)
	archive := insert("2026-03-02", "excused", "end_of_day", morning.Add(-time.Hour), &archivedAt)
	trip := insert("2026-03-02", "class_trip", "manual", morning, nil)
	sick := insert("2026-03-02", "sick", "parent", morning.Add(time.Hour), nil)
	alone := insert("2026-03-03", "sick", "parent", morning, nil)

	// The trip owned two blocks of the day: an open one moves to the kept
	// sick day, a completed one keeps its historical absence.
	room := testpkg.CreateTestRoom(t, db, "one-standing")
	openBlock := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2026, 3, 2), room.ID, testpkg.ActivityInstanceOpts{Title: "open"})
	doneBlock := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2026, 3, 2), room.ID, testpkg.ActivityInstanceOpts{Title: "done", StartHHMM: "15:00", EndHHMM: "16:00"})
	openSlot := testpkg.CreateTestInstanceStudent(t, db, openBlock.ID, student.ID, testpkg.ScheduleAttendanceStatusAbsent, testpkg.InstanceStudentOpts{StudentStatusDayID: &trip})
	doneSlot := testpkg.CreateTestInstanceStudent(t, db, doneBlock.ID, student.ID, testpkg.ScheduleAttendanceStatusAbsent, testpkg.InstanceStudentOpts{StudentStatusDayID: &trip})
	testpkg.SetActivityInstanceLifecycle(t, ctx, db, doneBlock.ID, "completed")

	require.NoError(t, studentStatusDaysOneStandingUp(ctx, db))
	require.NoError(t, studentStatusDaysOneStandingUp(ctx, db), "the migration reruns")

	type state struct {
		ClearedAt     *time.Time `bun:"cleared_at"`
		Source        string     `bun:"source"`
		ClearedSource *string    `bun:"cleared_source"`
	}
	read := func(id int64) state {
		t.Helper()
		var value state
		require.NoError(t, db.NewRaw(`SELECT cleared_at, source, cleared_source FROM active.student_status_days WHERE id = ?`, id).Scan(ctx, &value))
		return value
	}
	assert.Nil(t, read(sick).ClearedAt, "the latest report stands")
	assert.Nil(t, read(alone).ClearedAt, "a day with one status keeps it")
	replacedTrip := read(trip)
	require.NotNil(t, replacedTrip.ClearedAt)
	assert.True(t, replacedTrip.ClearedAt.Equal(morning.Add(time.Hour)), "cleared when the sick report came in")
	assert.Equal(t, "manual", replacedTrip.Source, "the replaced report keeps its source (#6842)")
	require.NotNil(t, replacedTrip.ClearedSource)
	assert.Equal(t, "parent", *replacedTrip.ClearedSource, "the kept report's source records the clearing")
	replacedArchive := read(archive)
	assert.Equal(t, "end_of_day", replacedArchive.Source)
	require.NotNil(t, replacedArchive.ClearedSource, "the archive no longer stands")
	require.NotNil(t, replacedArchive.ClearedAt)
	assert.True(t, replacedArchive.ClearedAt.Equal(archivedAt), "the archive keeps its own time")

	slot := func(id int64) (owner int64, substatus string) {
		t.Helper()
		require.NoError(t, db.NewRaw(`SELECT student_status_day_id, COALESCE(substatus, '') FROM active.activity_session_attendance WHERE instance_student_id = ?`, id).Scan(ctx, &owner, &substatus))
		return owner, substatus
	}
	owner, substatus := slot(openSlot.ID)
	assert.Equal(t, sick, owner, "the open block follows the kept status")
	assert.Equal(t, "sick", substatus)
	owner, _ = slot(doneSlot.ID)
	assert.Equal(t, trip, owner, "the completed block keeps its absence")

	_, err = db.ExecContext(ctx, `
		INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, '2026-03-03'::date, 'excused', NOW(), 'manual')`, tenantID, student.ID)
	require.ErrorContains(t, err, "uq_student_status_days_one_standing")
}
