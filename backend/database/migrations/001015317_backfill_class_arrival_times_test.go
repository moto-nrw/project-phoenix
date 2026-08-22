package migrations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func classArrivalTimesOf(t *testing.T, db *bun.DB, tenantID int64, class string) map[string]string {
	t.Helper()
	var raw []byte
	require.NoError(t, db.NewRaw(`
		SELECT arrival_times
		FROM education.class_arrival_times
		WHERE tenant_id = ? AND LOWER(BTRIM(school_class)) = LOWER(BTRIM(?))
	`, tenantID, class).Scan(context.Background(), &raw))
	times := map[string]string{}
	require.NoError(t, json.Unmarshal(raw, &times))
	return times
}

func storedArrivalTime(t *testing.T, db *bun.DB, rowID int64) *string {
	t.Helper()
	var hhmm *string
	require.NoError(t, db.NewRaw(`
		SELECT TO_CHAR(expected_arrival, 'HH24:MI')
		FROM schedule.student_arrival_schedules
		WHERE id = ?
	`, rowID).Scan(context.Background(), &hhmm))
	return hhmm
}

// TestBackfillClassArrivalTimes pins what the #2414 backfill promises: the
// common time of a class becomes the class timetable, the children carrying
// exactly that time start inheriting it, and a deviating child keeps its own.
func TestBackfillClassArrivalTimes(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	staff := testpkg.CreateTestStaff(t, db, "Backfill", "Betreuung")

	majorityA := testpkg.CreateTestStudent(t, db, "Mehr", "Heit", "5x")
	majorityB := testpkg.CreateTestStudent(t, db, "Mehr", "HeitZwei", "5x")
	deviating := testpkg.CreateTestStudent(t, db, "Ab", "Weichler", "5x")
	alumnus := testpkg.CreateTestStudent(t, db, "Alt", "Kind", "5x")
	_, err := db.ExecContext(context.Background(), `UPDATE users.students SET status = 'alumnus' WHERE id = ?`, alumnus.ID)
	require.NoError(t, err)

	rowA := testpkg.CreateTestArrivalSchedule(t, db, majorityA.ID, scheduleModels.WeekdayMonday, staff.ID, "11:45")
	rowB := testpkg.CreateTestArrivalSchedule(t, db, majorityB.ID, scheduleModels.WeekdayMonday, staff.ID, "11:45")
	rowDeviating := testpkg.CreateTestArrivalSchedule(t, db, deviating.ID, scheduleModels.WeekdayMonday, staff.ID, "12:15")
	rowTuesday := testpkg.CreateTestArrivalSchedule(t, db, majorityA.ID, scheduleModels.WeekdayTuesday, staff.ID, "13:30")
	rowAlumnus := testpkg.CreateTestArrivalSchedule(t, db, alumnus.ID, scheduleModels.WeekdayMonday, staff.ID, "11:45")

	require.NoError(t, backfillClassArrivalTimesUp(context.Background(), db))

	t.Run("the common time per weekday becomes the class timetable", func(t *testing.T) {
		times := classArrivalTimesOf(t, db, tenantID, "5x")
		require.NotNil(t, times)
		assert.Equal(t, "11:45", times["mon"])
		assert.Equal(t, "13:30", times["tue"])
	})

	t.Run("children carrying the class time start inheriting it", func(t *testing.T) {
		assert.Nil(t, storedArrivalTime(t, db, rowA.ID))
		assert.Nil(t, storedArrivalTime(t, db, rowB.ID))
		assert.Nil(t, storedArrivalTime(t, db, rowTuesday.ID))
	})

	t.Run("a deviating child keeps its own time", func(t *testing.T) {
		kept := storedArrivalTime(t, db, rowDeviating.ID)
		require.NotNil(t, kept)
		assert.Equal(t, "12:15", *kept)
	})

	t.Run("an alumnus keeps historical arrival times", func(t *testing.T) {
		kept := storedArrivalTime(t, db, rowAlumnus.ID)
		require.NotNil(t, kept)
		assert.Equal(t, "11:45", *kept)
	})

	t.Run("the care days themselves are untouched", func(t *testing.T) {
		var careDays int
		require.NoError(t, db.NewRaw(`
			SELECT COUNT(*) FROM schedule.student_arrival_schedules
			WHERE student_id IN (?, ?, ?)
		`, majorityA.ID, majorityB.ID, deviating.ID).Scan(context.Background(), &careDays))
		assert.Equal(t, 4, careDays)
	})
}
