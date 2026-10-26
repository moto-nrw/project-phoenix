package presenceprojectionhttp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The export carries the same "Angemeldet von" / "Abgemeldet von" as the
// history (#4761): a slot row takes them from the school stay that covers
// its check-in, an unassigned row from its own stay.
func TestAttendanceExportRows_CarryWhoRecordedTheStay(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, 9, 29)
	day := date.BerlinMidnight()
	checkIn, checkOut := day.Add(8*time.Hour), day.Add(15*time.Hour)
	var staffOut, device int64 = 12, 3
	web := &studentpresence.Attendance{Date: date.String(), CheckInTime: checkIn, CheckOutTime: &checkOut, CheckedInBy: 11, CheckedOutBy: &staffOut}
	slot := &studentpresence.HistorySlot{
		Instance: &studentpresence.HistorySlotInstance{
			Date: date, Title: "Mittagsbetreuung",
			StartTime: time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC), EndTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		},
		Attendance: &studentpresence.HistorySlotAttendance{Status: studentpresence.SessionAttendancePresent, CheckedInAt: &checkIn},
	}
	names := studentpresence.NewAttendanceRecorders(map[int64]string{11: "Wiebke Webbuchung", 12: "Paul Abholung"})

	rows := attendanceExportRows([]*studentpresence.HistorySlot{slot}, []*studentpresence.Attendance{web}, names)
	require.Len(t, rows, 1, "the stay is covered by its slot")
	assert.Equal(t, "Wiebke Webbuchung", rows[0].Values[attendanceColumnCheckedIn])
	assert.Equal(t, "Paul Abholung", rows[0].Values[attendanceColumnCheckedOut])

	kiosk := &studentpresence.Attendance{Date: date.String(), CheckInTime: checkIn, CheckOutTime: &checkOut, CheckedOutDeviceID: &device}
	open := &studentpresence.Attendance{Date: date.String(), CheckInTime: checkIn, CheckedInBy: 13}
	automatic := &studentpresence.Attendance{Date: date.String(), CheckInTime: checkIn, CheckOutTime: &checkOut, CheckedInBy: 13}
	for _, tc := range []struct {
		name    string
		stay    *studentpresence.Attendance
		in, out string
	}{
		{name: "kiosk without a person", stay: kiosk, in: "am Gerät", out: "am Gerät"},
		{name: "erased staff, still present", stay: open, in: "Ehemalige Mitarbeitende", out: "–"},
		{name: "automatic closing", stay: automatic, in: "Ehemalige Mitarbeitende", out: "automatisch"},
	} {
		unassigned := attendanceExportRows(nil, []*studentpresence.Attendance{tc.stay}, names)
		require.Len(t, unassigned, 1, tc.name)
		assert.Equal(t, tc.in, unassigned[0].Values[attendanceColumnCheckedIn], tc.name)
		assert.Equal(t, tc.out, unassigned[0].Values[attendanceColumnCheckedOut], tc.name)
	}

	// A slot row outside every stay names nobody.
	orphan := attendanceExportRows([]*studentpresence.HistorySlot{slot}, nil, names)
	require.Len(t, orphan, 1)
	assert.Equal(t, "–", orphan[0].Values[attendanceColumnCheckedIn])
	assert.Equal(t, "–", orphan[0].Values[attendanceColumnCheckedOut])
}

func TestAttendanceExportColumns_NameWhoRecorded(t *testing.T) {
	t.Parallel()
	for _, columns := range [][]lists.ColumnID{columnIDs(attendanceExportColumns()), columnIDs(attendanceSessionExportColumns())} {
		assert.Contains(t, columns, attendanceColumnCheckedIn)
		assert.Contains(t, columns, attendanceColumnCheckedOut)
	}
}

func TestAttendanceExportRowsWithoutActors_LeavesActorColumnsEmpty(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, 9, 29)
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	rows := attendanceExportRowsWithoutActors(nil, []*studentpresence.Attendance{{
		Date: date.String(), CheckInTime: checkIn, CheckedInBy: 11,
	}})

	require.Len(t, rows, 1)
	assert.NotContains(t, rows[0].Values, attendanceColumnCheckedIn)
	assert.NotContains(t, rows[0].Values, attendanceColumnCheckedOut)
}
