package classdayhttp_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The class day reads the status days that stand for the day (#6291): a day
// the end-of-day run archived still tells the Lehrkraft why the child is not
// there, with the school as the source when the school reported it, while a
// day someone withdrew stays gone (#4793).
func TestDayReportShowsTheStatusDaysTheEndOfDayRunArchived(t *testing.T) {
	t.Parallel()
	f := setupMorningAttendanceFixture(t, morningMonday)
	date, err := calendar.ParseDate(morningMonday)
	require.NoError(t, err)
	endOfDay := date.BerlinMidnight().Add(18 * time.Hour)
	withdrawn := testpkg.CreateTestStudent(t, f.db, "Wilma", "Zurück", f.class)

	// Rows as the archive left them before #6291: cleared at the end of the
	// day, with the end-of-day source in place of the school's.
	archived := func(studentID int64, status, source string) {
		t.Helper()
		_, err := f.db.ExecContext(testpkg.Ctx(t), `INSERT INTO active.student_status_days
			(tenant_id, student_id, date, status, reported_at, cleared_at, source) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			f.tenantID, studentID, date, status, date.BerlinMidnight().Add(8*time.Hour), endOfDay, source)
		require.NoError(t, err)
	}
	archived(f.present, "excused", "end_of_day")
	archived(withdrawn.ID, "sick", "manual")

	_, rows := f.absenceRows(t, classDayReadPermission)
	assert.Equal(t, "excused", rows[f.present].Status, "the archived day still explains the absence")
	assert.Equal(t, "sick", rows[f.sick].Status, "an active day is unchanged")
	assert.Empty(t, rows[withdrawn.ID].Status, "a withdrawn day stays withdrawn")
}
