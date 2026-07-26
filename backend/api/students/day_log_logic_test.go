package students

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dayLogAttendanceRow(checkIn time.Time, checkOut *time.Time) *active.Attendance {
	return &active.Attendance{StudentID: 1, Date: timezone.TodayDate(), CheckInTime: checkIn, CheckOutTime: checkOut}
}

func dayLogStatusRow(status, source string, reportedAt time.Time) *active.StudentStatusDay {
	return &active.StudentStatusDay{StudentID: 1, Date: timezone.TodayDate(), Status: status, Source: source, ReportedAt: reportedAt}
}

func TestClassifyDayLogStudent_PresentWinsAndCarriesHint(t *testing.T) {
	now := time.Now()
	checkOut := now.Add(6 * time.Hour)
	row := dayLogStudent{}
	classifyDayLogStudent(&row,
		[]*active.Attendance{dayLogAttendanceRow(now, &checkOut)},
		[]*active.StudentStatusDay{dayLogStatusRow(active.StudentStatusDaySick, "parent", now)},
		scheduleService.CareDayScheduled,
	)
	assert.Equal(t, dayLogStatusPresent, row.Status)
	assert.Equal(t, "Krankmeldung liegt vor", row.Hint)
	require.NotNil(t, row.CheckInTime)
	require.NotNil(t, row.CheckOutTime)
}

func TestClassifyDayLogStudent_SickBeatsExcused(t *testing.T) {
	now := time.Now()
	row := dayLogStudent{}
	classifyDayLogStudent(&row, nil, []*active.StudentStatusDay{
		dayLogStatusRow(active.StudentStatusDayExcused, "manual", now.Add(time.Hour)),
		dayLogStatusRow(active.StudentStatusDaySick, "parent", now),
	}, "")
	assert.Equal(t, dayLogStatusSick, row.Status)
	assert.Equal(t, "parent", row.Source)
	require.NotNil(t, row.ReportedAt)
}

func TestClassifyDayLogStudent_CancelledCareDayIsSignedOff(t *testing.T) {
	row := dayLogStudent{}
	classifyDayLogStudent(&row, nil, nil, scheduleService.CareDayCancelled)
	assert.Equal(t, dayLogStatusExcused, row.Status)
	assert.Equal(t, dayLogSourceCancelledCareDay, row.Source)
	assert.Equal(t, "Abgemeldet", dayLogStatusLabel(row.Status, row.Source))
}

func TestClassifyDayLogStudent_NotScheduledAndAbsent(t *testing.T) {
	row := dayLogStudent{}
	classifyDayLogStudent(&row, nil, nil, scheduleService.CareDayNotScheduled)
	assert.Equal(t, dayLogStatusNotScheduled, row.Status)
	assert.Equal(t, "Nicht eingeplant", dayLogStatusLabel(row.Status, ""))

	row = dayLogStudent{}
	classifyDayLogStudent(&row, nil, nil, scheduleService.CareDayUnknown)
	assert.Equal(t, dayLogStatusAbsent, row.Status)
	assert.Equal(t, "Abwesend", dayLogStatusLabel(row.Status, ""))
}

// The listexport renderers treat a row with GroupTitle as a section MARKER and
// never render its Values — data rows must not carry a GroupTitle (the first
// PDF render shipped empty because every row did).
func TestBuildDayLogExportDocument_UsesMarkerRowConvention(t *testing.T) {
	multi := dayLogResponse{Groups: []dayLogGroup{
		{Name: "Gruppe A", Students: []dayLogStudent{{FirstName: "Mia", LastName: "Bauer", Status: dayLogStatusPresent, Label: "Anwesend"}}},
		{Name: "Gruppe B", Students: []dayLogStudent{{FirstName: "Paul", LastName: "Wolf", Status: dayLogStatusAbsent, Label: "Abwesend"}}},
	}}
	doc := buildDayLogExportDocument(multi, timezone.TodayDate())
	require.Len(t, doc.Rows, 4)
	assert.Equal(t, "Gruppe A", doc.Rows[0].GroupTitle)
	assert.Empty(t, doc.Rows[0].Values, "marker rows carry no values")
	assert.Empty(t, doc.Rows[1].GroupTitle, "data rows carry no group title")
	assert.Equal(t, "Bauer, Mia", doc.Rows[1].Values["name"])
	assert.Equal(t, "Gruppe B", doc.Rows[2].GroupTitle)
	assert.Equal(t, "Wolf, Paul", doc.Rows[3].Values["name"])

	single := dayLogResponse{Groups: multi.Groups[:1]}
	doc = buildDayLogExportDocument(single, timezone.TodayDate())
	require.Len(t, doc.Rows, 1)
	assert.Empty(t, doc.Rows[0].GroupTitle, "single-group export stays ungrouped")
	assert.Equal(t, "Bauer, Mia", doc.Rows[0].Values["name"])
}

func TestMergeDayLogAttendance_OpenSessionKeepsDepartureOpen(t *testing.T) {
	morning := time.Now().Add(-6 * time.Hour)
	noon := morning.Add(4 * time.Hour)
	noonOut := morning.Add(3 * time.Hour)

	checkIn, checkOut := mergeDayLogAttendance([]*active.Attendance{
		dayLogAttendanceRow(noon, nil), // re-entry, still checked in
		dayLogAttendanceRow(morning, &noonOut),
	})
	assert.Equal(t, morning, checkIn, "earliest arrival wins")
	assert.Nil(t, checkOut, "open session keeps the day open")

	checkIn, checkOut = mergeDayLogAttendance([]*active.Attendance{
		dayLogAttendanceRow(morning, &noonOut),
		dayLogAttendanceRow(noon, &noon),
	})
	assert.Equal(t, morning, checkIn)
	require.NotNil(t, checkOut)
	assert.Equal(t, noon, *checkOut, "latest departure wins")
}
