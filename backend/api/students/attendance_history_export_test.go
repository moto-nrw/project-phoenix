package students

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAttendanceExportOptions_RejectsFutureDates(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 8, 24)
	future := today.AddDays(1)
	tests := []struct {
		name  string
		query string
	}{
		{name: "future from", query: "?from=" + future.String() + "&to=" + future.String()},
		{name: "future to", query: "?from=" + today.String() + "&to=" + future.String()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/attendance-history/export"+tt.query, nil)
			_, err := parseAttendanceExportOptions(req, 30, today)
			require.EqualError(t, err, "attendance exports cannot include future dates")
		})
	}
}

func TestParseAttendanceExportOptions_AcceptsToday(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 8, 24)
	req := httptest.NewRequest("GET", "/attendance-history/export?from="+today.String()+"&to="+today.String(), nil)
	options, err := parseAttendanceExportOptions(req, 30, today)
	require.NoError(t, err)
	assert.Equal(t, today, options.From)
	assert.Equal(t, today, options.To)
}

func TestAttendanceExportRows_KeepSlotsAndExplicitUnassignedSession(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 15)
	morningCheckIn := time.Date(2026, 7, 15, 7, 0, 0, 0, timezone.Berlin)
	unassignedCheckIn := time.Date(2026, 7, 15, 9, 0, 0, 0, timezone.Berlin)
	sick := scheduleModel.AttendanceSubstatusSick
	rows := attendanceExportRows([]*scheduleModel.ScheduledInstanceRow{
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: scheduleModel.Date(date), Title: "Morgenbetreuung",
				StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
			},
			Attendance: &scheduleModel.InstanceStudent{
				Status: scheduleModel.AttendanceStatusPresent, CheckedInAt: &morningCheckIn,
			},
		},
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: scheduleModel.Date(date), Title: "Nachmittagsbetreuung",
				StartTime: time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
			},
			Attendance: &scheduleModel.InstanceStudent{
				Status: scheduleModel.AttendanceStatusAbsent, Substatus: &sick,
			},
		},
	}, []*activeModel.Attendance{
		{Date: date, CheckInTime: morningCheckIn},
		{Date: date, CheckInTime: unassignedCheckIn},
	})

	require.Len(t, rows, 3)
	// Chronological within the day: 07:00 slot, 09:00 unassigned session, 12:00 slot.
	assert.Equal(t, "Morgenbetreuung", rows[0].Values[attendanceColumnOffering])
	assert.Equal(t, "Anwesend", rows[0].Values[attendanceColumnStatus])
	assert.Equal(t, "Ohne Zuordnung", rows[1].Values[attendanceColumnOffering])
	assert.Equal(t, "Nicht zugeordnet", rows[1].Values[attendanceColumnAssignment])
	assert.Equal(t, "Nachmittagsbetreuung", rows[2].Values[attendanceColumnOffering])
	assert.Equal(t, "Krank", rows[2].Values[attendanceColumnStatus])
}

// TestAttendanceExportRows_SameSlotReentryIsCoveredByWindow: re-entry into the
// same care slot re-stamps the slot's checked_in_at, so the earlier session
// loses its exact timestamp match. The scheduled-window fallback must still
// claim it — no false "Nicht zugeordnet" row for booked presence.
func TestAttendanceExportRows_SameSlotReentryIsCoveredByWindow(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 15)
	firstCheckIn := time.Date(2026, 7, 15, 8, 0, 0, 0, timezone.Berlin)
	reentryCheckIn := time.Date(2026, 7, 15, 10, 0, 0, 0, timezone.Berlin)

	rows := attendanceExportRows([]*scheduleModel.ScheduledInstanceRow{
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: scheduleModel.Date(date), Title: "Ganztagsbetreuung",
				StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
			},
			Attendance: &scheduleModel.InstanceStudent{
				// Reopened slot: checked_in_at re-stamped with the re-entry.
				Status: scheduleModel.AttendanceStatusPresent, CheckedInAt: &reentryCheckIn,
			},
		},
	}, []*activeModel.Attendance{
		{Date: date, CheckInTime: firstCheckIn},
		{Date: date, CheckInTime: reentryCheckIn},
	})

	require.Len(t, rows, 1, "both sessions belong to the booked slot — no unassigned rows")
	assert.Equal(t, "Ganztagsbetreuung", rows[0].Values[attendanceColumnOffering])
	assert.Equal(t, "Gebucht", rows[0].Values[attendanceColumnAssignment])
}

func TestAttendanceExportDocument_RendersEverySupportedFormat(t *testing.T) {
	t.Parallel()

	renderer := listexport.NewService()
	doc := listexport.Document{
		Title: "Anwesenheit je Betreuungsangebot", GeneratedAt: time.Now(),
		Columns: attendanceExportColumns(),
		Rows: []listexport.Row{{Values: map[listexport.ColumnID]string{
			attendanceColumnDate: "15.07.2026", attendanceColumnOffering: "Morgenbetreuung",
		}}},
	}
	for _, format := range []listexport.Format{listexport.FormatPDF, listexport.FormatDOCX, listexport.FormatXLSX} {
		t.Run(string(format), func(t *testing.T) {
			file, err := renderer.Render(doc, format, "anwesenheit")
			require.NoError(t, err)
			assert.NotEmpty(t, file.Data)
			assert.True(t, strings.HasSuffix(file.Filename, "."+string(format)))
		})
	}
}

// TestAttendanceSessionExportColumns_OmitsPlanColumns: without a care plan the
// export lists plain sessions — no "Betreuungsangebot" and no "Zuordnung"
// column, so no row can read as unassigned.
func TestAttendanceSessionExportColumns_OmitsPlanColumns(t *testing.T) {
	t.Parallel()

	ids := make([]listexport.ColumnID, 0, 5)
	for _, column := range attendanceSessionExportColumns() {
		ids = append(ids, column.ID)
	}
	assert.NotContains(t, ids, attendanceColumnOffering)
	assert.NotContains(t, ids, attendanceColumnAssignment)
	assert.Equal(t, []listexport.ColumnID{
		attendanceColumnDate, attendanceColumnWindow, attendanceColumnStatus,
		attendanceColumnCheckIn, attendanceColumnCheckOut,
	}, ids)
}

// TestAttendanceExportRows_KeepsSessionsWithoutAnyPlan: the plan-free document
// still lists every observed session — the neutral wording comes from dropping
// the offering/assignment columns, not from dropping rows.
func TestAttendanceExportRows_KeepsSessionsWithoutAnyPlan(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 7, 15)
	checkIn := time.Date(2026, 7, 15, 9, 0, 0, 0, timezone.Berlin)
	checkOut := time.Date(2026, 7, 15, 16, 0, 0, 0, timezone.Berlin)

	rows := attendanceExportRows(nil, []*activeModel.Attendance{
		{Date: date, CheckInTime: checkIn, CheckOutTime: &checkOut},
	})

	require.Len(t, rows, 1)
	assert.Equal(t, "09:00", rows[0].Values[attendanceColumnCheckIn])
	assert.Equal(t, "16:00", rows[0].Values[attendanceColumnCheckOut])
}

func TestAttendanceExportRows_SortsChronologicallyAcrossSources(t *testing.T) {
	t.Parallel()

	day1 := timezone.NewDate(2026, 7, 14)
	day2 := timezone.NewDate(2026, 7, 15)
	day1Unassigned := time.Date(2026, 7, 14, 9, 30, 0, 0, timezone.Berlin)
	day2CheckIn := time.Date(2026, 7, 15, 12, 5, 0, 0, timezone.Berlin)

	rows := attendanceExportRows([]*scheduleModel.ScheduledInstanceRow{
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: scheduleModel.Date(day2), Title: "Nachmittagsbetreuung",
				StartTime: time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC),
			},
			Attendance: &scheduleModel.InstanceStudent{
				Status: scheduleModel.AttendanceStatusPresent, CheckedInAt: &day2CheckIn,
			},
		},
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: scheduleModel.Date(day1), Title: "Morgenbetreuung",
				StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
			},
			Attendance: &scheduleModel.InstanceStudent{Status: scheduleModel.AttendanceStatusAbsent},
		},
	}, []*activeModel.Attendance{
		{Date: day1, CheckInTime: day1Unassigned},
	})

	require.Len(t, rows, 3)
	assert.Equal(t, "14.07.2026", rows[0].Values[attendanceColumnDate])
	assert.Equal(t, "Morgenbetreuung", rows[0].Values[attendanceColumnOffering])
	assert.Equal(t, "14.07.2026", rows[1].Values[attendanceColumnDate])
	assert.Equal(t, "Ohne Zuordnung", rows[1].Values[attendanceColumnOffering])
	assert.Equal(t, "15.07.2026", rows[2].Values[attendanceColumnDate])
	assert.Equal(t, "Nachmittagsbetreuung", rows[2].Values[attendanceColumnOffering])
}
