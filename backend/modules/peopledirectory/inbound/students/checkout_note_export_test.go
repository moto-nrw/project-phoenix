package students

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Early-checkout note (#3324) in the Tagesauswertung and the per-child
// attendance export.

func checkoutNoteStay(checkIn, checkOut time.Time, note string) *studentpresence.Attendance {
	row := &studentpresence.Attendance{Date: "2026-09-29", CheckInTime: checkIn, CheckOutTime: &checkOut}
	if note != "" {
		row.CheckOutNote = &note
	}
	return row
}

func TestDayLogCheckOutNote_KeepsEarlyNoteAcrossReturnInArrivalOrder(t *testing.T) {
	t.Parallel()

	day := timezone.NewDate(2026, 9, 29).BerlinMidnight()
	// Stays arrive out of order: the second stay (no note) is listed first.
	rows := []*studentpresence.Attendance{
		checkoutNoteStay(day.Add(14*time.Hour), day.Add(16*time.Hour), ""),
		checkoutNoteStay(day.Add(8*time.Hour), day.Add(11*time.Hour), "Arzttermin"),
		checkoutNoteStay(day.Add(16*time.Hour+30*time.Minute), day.Add(17*time.Hour), "Oma holt ab"),
	}

	assert.Equal(t, "Arzttermin · Oma holt ab", dayLogCheckOutNote(rows))
	assert.Empty(t, dayLogCheckOutNote(rows[:1]))
}

func TestClassifyDayLogStudent_CarriesCheckOutNote(t *testing.T) {
	t.Parallel()

	day := timezone.NewDate(2026, 9, 29).BerlinMidnight()
	row := &dayLogStudent{}
	classifyDayLogStudent(row, []*studentpresence.Attendance{
		checkoutNoteStay(day.Add(8*time.Hour), day.Add(11*time.Hour), "Arzttermin"),
	}, nil, "")

	assert.Equal(t, dayLogStatusPresent, row.Status)
	assert.Equal(t, "Arzttermin", row.CheckOutNote)
}

func TestDayLogExportDetails_ShowsCheckOutNote(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Früher gegangen: Arzttermin", dayLogExportDetails(dayLogStudent{CheckOutNote: "Arzttermin"}))
	assert.Equal(t, "Krankmeldung vorhanden · Früher gegangen: Arzttermin",
		dayLogExportDetails(dayLogStudent{Hint: "Krankmeldung vorhanden", CheckOutNote: "Arzttermin"}))
	assert.Equal(t, "Krankmeldung vorhanden", dayLogExportDetails(dayLogStudent{Hint: "Krankmeldung vorhanden"}))
}

func TestAttendanceExportRows_CarryCheckOutNote(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 9, 29)
	day := date.BerlinMidnight()
	checkIn := day.Add(8 * time.Hour)
	checkOut := day.Add(11 * time.Hour)
	slotStart := time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC)
	slotEnd := time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC)

	// A slot closed by the noted checkout finds the note by its instant.
	slot := &studentpresence.HistorySlot{
		Instance: &studentpresence.HistorySlotInstance{Date: date, StartTime: slotStart, EndTime: slotEnd, Title: "Frühbetreuung"},
		Attendance: &studentpresence.HistorySlotAttendance{
			Status:       studentpresence.SessionAttendancePresent,
			CheckedInAt:  &checkIn,
			CheckedOutAt: &checkOut,
		},
	}
	noted := checkoutNoteStay(checkIn, checkOut, "Arzttermin")

	rows := attendanceExportRows([]*studentpresence.HistorySlot{slot}, []*studentpresence.Attendance{noted})
	require.Len(t, rows, 1, "the stay is covered by its slot")
	assert.Equal(t, "Arzttermin", rows[0].Values[attendanceColumnNote])

	// A stay no slot claims carries its own note.
	unassigned := attendanceExportRows(nil, []*studentpresence.Attendance{noted})
	require.Len(t, unassigned, 1)
	assert.Equal(t, "Arzttermin", unassigned[0].Values[attendanceColumnNote])

	// Both column sets render the note.
	assert.Contains(t, columnIDs(attendanceExportColumns()), attendanceColumnNote)
	assert.Contains(t, columnIDs(attendanceSessionExportColumns()), attendanceColumnNote)
}

func columnIDs(columns []lists.Column) []lists.ColumnID {
	ids := make([]lists.ColumnID, 0, len(columns))
	for _, column := range columns {
		ids = append(ids, column.ID)
	}
	return ids
}
