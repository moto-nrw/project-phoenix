package students

import (
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

func TestAttendanceExportRows_KeepSlotsAndExplicitUnassignedSession(t *testing.T) {
	date := timezone.NewDate(2026, 7, 15)
	morningCheckIn := time.Date(2026, 7, 15, 7, 0, 0, 0, timezone.Berlin)
	unassignedCheckIn := time.Date(2026, 7, 15, 9, 0, 0, 0, timezone.Berlin)
	sick := scheduleModel.AttendanceSubstatusSick
	rows := attendanceExportRows([]*scheduleModel.ScheduledInstanceRow{
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: date, Title: "Morgenbetreuung",
				StartTime: time.Date(1, 1, 1, 7, 0, 0, 0, time.UTC),
				EndTime:   time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
			},
			Attendance: &scheduleModel.InstanceStudent{
				Status: scheduleModel.AttendanceStatusPresent, CheckedInAt: &morningCheckIn,
			},
		},
		{
			Instance: &scheduleModel.ActivityInstance{
				Date: date, Title: "Nachmittagsbetreuung",
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
	assert.Equal(t, "Morgenbetreuung", rows[0].Values[attendanceColumnOffering])
	assert.Equal(t, "Anwesend", rows[0].Values[attendanceColumnStatus])
	assert.Equal(t, "Nachmittagsbetreuung", rows[1].Values[attendanceColumnOffering])
	assert.Equal(t, "Krank", rows[1].Values[attendanceColumnStatus])
	assert.Equal(t, "Ohne Zuordnung", rows[2].Values[attendanceColumnOffering])
	assert.Equal(t, "Ungeplant, ohne Buchung", rows[2].Values[attendanceColumnAssignment])
}

func TestAttendanceExportDocument_RendersEverySupportedFormat(t *testing.T) {
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
