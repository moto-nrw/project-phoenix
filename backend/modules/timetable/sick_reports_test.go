package timetable

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
)

// CoversFullDay decides the plan days of a sick report for the cascade and
// the materializer alike: inside the range, minus a half start or end day.
func TestSickReportCoversFullDay(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.September, 7)
	wednesday := monday.AddDays(2)
	report := SickReport{DateStart: monday, DateEnd: wednesday}
	for _, tc := range []struct {
		name               string
		skipStart, skipEnd bool
		want               map[calendar.Date]bool
	}{
		{name: "full range", want: map[calendar.Date]bool{monday.AddDays(-1): false, monday: true, monday.AddDays(1): true, wednesday: true, wednesday.AddDays(1): false}},
		{name: "half start day", skipStart: true, want: map[calendar.Date]bool{monday: false, monday.AddDays(1): true, wednesday: true}},
		{name: "half end day", skipEnd: true, want: map[calendar.Date]bool{monday: true, monday.AddDays(1): true, wednesday: false}},
		{name: "both half", skipStart: true, skipEnd: true, want: map[calendar.Date]bool{monday: false, monday.AddDays(1): true, wednesday: false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := report
			report.SkipStartDay, report.SkipEndDay = tc.skipStart, tc.skipEnd
			for day, want := range tc.want {
				assert.Equal(t, want, report.CoversFullDay(day), day.String())
			}
		})
	}
}
