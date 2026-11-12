package presenceprojectionhttp

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func checkoutNoteStay(checkIn, checkOut time.Time, note string) *studentpresence.Attendance {
	row := &studentpresence.Attendance{Date: "2026-09-29", CheckInTime: checkIn, CheckOutTime: &checkOut}
	if note != "" {
		row.CheckOutNote = &note
	}
	return row
}

func TestDayLogCheckOutNote_KeepsEarlyNoteAcrossReturnInArrivalOrder(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, 9, 29).BerlinMidnight()
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

	day := calendar.NewDate(2026, 9, 29).BerlinMidnight()
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

	date := calendar.NewDate(2026, 9, 29)
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

	rows := attendanceExportRows([]*studentpresence.HistorySlot{slot}, []*studentpresence.Attendance{noted}, studentpresence.NewAttendanceRecorders(nil))
	require.Len(t, rows, 1, "the stay is covered by its slot")
	assert.Equal(t, "Arzttermin", rows[0].Values[attendanceColumnNote])

	// A stay no slot claims carries its own note.
	unassigned := attendanceExportRows(nil, []*studentpresence.Attendance{noted}, studentpresence.NewAttendanceRecorders(nil))
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

func classStay(checkIn, checkOut time.Time) *studentpresence.Attendance {
	row := checkoutNoteStay(checkIn, checkOut, "")
	reason := studentpresence.CheckoutReasonClass
	row.CheckOutReason = &reason
	return row
}

func TestDayLogClassPeriods_ReadsTheGapBetweenStays(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, 9, 29).BerlinMidnight()
	morning := classStay(day.Add(7*time.Hour), day.Add(8*time.Hour))
	afternoon := checkoutNoteStay(day.Add(12*time.Hour), day.Add(15*time.Hour), "")
	// Stays arrive out of order: the later one is listed first.
	periods := dayLogClassPeriods([]*studentpresence.Attendance{afternoon, morning})
	require.Len(t, periods, 1)
	assert.Equal(t, day.Add(8*time.Hour), periods[0].From)
	require.NotNil(t, periods[0].Until)
	assert.Equal(t, day.Add(12*time.Hour), *periods[0].Until)
	assert.Equal(t, "Im Unterricht 08:00–12:00", dayLogExportDetails(dayLogStudent{ClassPeriods: periods}))

	// No return yet: the period is open, and the export says so.
	open := dayLogClassPeriods([]*studentpresence.Attendance{morning})
	require.Len(t, open, 1)
	assert.Nil(t, open[0].Until)
	assert.Equal(t, "Im Unterricht ab 08:00", dayLogExportDetails(dayLogStudent{ClassPeriods: open}))

	// Going home carries no class period.
	assert.Empty(t, dayLogClassPeriods([]*studentpresence.Attendance{afternoon}))
}

func TestAttendanceExport_TellsClassFromGoingHome(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, 9, 29).BerlinMidnight()
	notes := checkOutNotesByInstant([]*studentpresence.Attendance{
		classStay(day.Add(7*time.Hour), day.Add(8*time.Hour)),
		checkoutNoteStay(day.Add(12*time.Hour), day.Add(15*time.Hour), ""),
	})

	assert.Equal(t, attendanceWentToClassNote, notes[day.Add(8*time.Hour).UnixNano()])
	assert.NotContains(t, notes, day.Add(15*time.Hour).UnixNano())
	row := unassignedExportRow(classStay(day.Add(7*time.Hour), day.Add(8*time.Hour)))
	assert.Equal(t, attendanceWentToClassNote, row.Values[attendanceColumnNote])
}

func TestExportStatus_InClassIsItsOwnBucket(t *testing.T) {
	t.Parallel()

	student := StudentResponse{Location: studentpresence.InClassLocationLabel}
	assert.Equal(t, "unterricht", exportStatus(student))
	assert.Equal(t, "Im Unterricht", exportStatusLabel("unterricht"))
	assert.True(t, matchesExportStatusFilter(student, "unterricht"))
	assert.False(t, matchesExportStatusFilter(student, "abwesend"))
	assert.Contains(t, locationDerivedExportStatuses, "unterricht")
}

func TestApplyActualTimes_ClassIsNoPickup(t *testing.T) {
	t.Parallel()

	checkIn := time.Date(2026, 9, 29, 5, 30, 0, 0, time.UTC)
	checkOut := checkIn.Add(time.Hour)
	reason := studentpresence.CheckoutReasonClass
	response := StudentResponse{}
	applyActualTimesFromAttendance(&response, &studentpresence.DailyAttendanceStatus{
		Status: "checked_out", CheckInTime: &checkIn, CheckOutTime: &checkOut, CheckOutReason: &reason,
	})
	require.NotNil(t, response.ActualArrivalTime)
	assert.Nil(t, response.ActualPickupTime)
}

func TestApplyAtSchoolLocation_InClassEndsWithTheCareDay(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 29, 15, 0, 0, 0, calendar.Berlin)
	since := now.Add(-6 * time.Hour)
	responses := []StudentResponse{{ID: 1, HasFullAccess: true, Location: studentpresence.InClassLocationLabel, LocationSince: &since}}
	applyAtSchoolLocation(responses, nil, nil, "16:00", now)
	assert.Equal(t, studentpresence.InClassLocationLabel, responses[0].Location)

	applyAtSchoolLocation(responses, nil, nil, "16:00", now.Add(time.Hour))
	assert.Equal(t, studentpresence.AbsentLocationLabel, responses[0].Location)
	assert.Nil(t, responses[0].LocationSince, "the class checkout is not when the child got home")
}

func TestAttendanceExport_ReturnIntoTheSameSlotKeepsTheClassStay(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, 9, 29)
	midnight := day.BerlinMidnight()
	at := func(hour, minute int) time.Time {
		return midnight.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
	}
	checkedIn, checkedOut := at(12, 30), at(14, 0)
	// The return re-stamped the one all-day slot: it no longer shows the morning.
	slot := &studentpresence.HistorySlot{
		Instance: &studentpresence.HistorySlotInstance{
			Date: day, Title: "Betreuung", StartTime: at(8, 0), EndTime: at(16, 0),
		},
		Attendance: &studentpresence.HistorySlotAttendance{
			Status: studentpresence.SessionAttendancePresent, CheckedInAt: &checkedIn, CheckedOutAt: &checkedOut,
		},
	}
	morning := classStay(at(8, 0), at(11, 0))
	morning.Date = day.String()
	afternoon := checkoutNoteStay(checkedIn, checkedOut, "")
	afternoon.Date = day.String()

	rows := attendanceExportRowsWithoutActors([]*studentpresence.HistorySlot{slot}, []*studentpresence.Attendance{morning, afternoon})

	require.Len(t, rows, 2, "the class stay keeps its own row, the afternoon stays on the slot")
	// Rows sort by their window start: the slot and the stay both start at 08:00.
	assert.Equal(t, "Betreuung", rows[0].Values[attendanceColumnOffering])
	assert.Empty(t, rows[0].Values[attendanceColumnNote])
	assert.Equal(t, attendanceWentToClassNote, rows[1].Values[attendanceColumnNote])
	assert.Equal(t, "08:00", rows[1].Values[attendanceColumnCheckIn])
	assert.Equal(t, "11:00", rows[1].Values[attendanceColumnCheckOut])
}

func TestAttendanceExport_ClassStayOnItsSlotGetsNoSecondRow(t *testing.T) {
	t.Parallel()

	day := calendar.NewDate(2026, 9, 29)
	midnight := day.BerlinMidnight()
	checkedIn, checkedOut := midnight.Add(8*time.Hour), midnight.Add(11*time.Hour)
	slot := &studentpresence.HistorySlot{
		Instance: &studentpresence.HistorySlotInstance{
			Date: day, Title: "Betreuung", StartTime: midnight.Add(8 * time.Hour), EndTime: midnight.Add(16 * time.Hour),
		},
		Attendance: &studentpresence.HistorySlotAttendance{
			Status: studentpresence.SessionAttendancePresent, CheckedInAt: &checkedIn, CheckedOutAt: &checkedOut,
		},
	}
	stay := classStay(checkedIn, checkedOut)
	stay.Date = day.String()

	rows := attendanceExportRowsWithoutActors([]*studentpresence.HistorySlot{slot}, []*studentpresence.Attendance{stay})

	require.Len(t, rows, 1)
	assert.Equal(t, attendanceWentToClassNote, rows[0].Values[attendanceColumnNote])
}
