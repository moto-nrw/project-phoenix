package application

import (
	"testing"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spontaneousBlock(id int64, date calendar.Date, startHour, startMinute int, spontaneous bool) *scheduleModels.ActivityInstance {
	instance := &scheduleModels.ActivityInstance{Date: calendar.Date(date), Title: "Hausaufgaben"}
	instance.ID = id
	instance.IsSpontaneous = spontaneous
	instance.StartTime = time.Date(1, time.January, 1, startHour, startMinute, 0, 0, time.UTC)
	instance.EndTime = instance.StartTime.Add(time.Hour)
	return instance
}

func berlinAt(date calendar.Date, hour, minute int) time.Time {
	return date.BerlinMidnight().Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// A spontaneous block ends when its session ends, not at the placeholder hour
// the kiosk mirror wrote (#3921).
func TestSpontaneousCompletionEnd(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, 10, 9)
	cases := []struct {
		name        string
		instance    *scheduleModels.ActivityInstance
		completedAt time.Time
		want        string
		ok          bool
	}{
		{"completion clock", spontaneousBlock(1, day, 15, 16, true), berlinAt(day, 20, 32), "20:32", true},
		{"never before the start", spontaneousBlock(2, day, 15, 16, true), berlinAt(day, 15, 16), "15:17", true},
		{"later day ends at 23:59", spontaneousBlock(3, day, 22, 0, true), berlinAt(day.AddDays(1), 0, 30), "23:59", true},
		{"planned blocks keep their end", spontaneousBlock(4, day, 15, 16, false), berlinAt(day, 20, 32), "", false},
		{"no minute left after 23:59", spontaneousBlock(5, day, 23, 59, true), berlinAt(day, 23, 59), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			end, ok := SpontaneousCompletionEnd(tc.instance, tc.completedAt)
			require.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, end.Format("15:04"))
			}
		})
	}
}
