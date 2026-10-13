package timetracking

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeTrackingShiftUsesCalendarDateAndBerlinWallClock(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		day     Date
		utcHour int
	}{
		{NewDate(2026, 3, 28), 8},
		{NewDate(2026, 3, 29), 7},
		{NewDate(2026, 10, 25), 8},
	} {
		t.Run(test.day.String(), func(t *testing.T) {
			shift := &TimeTrackingShift{
				Date:      test.day,
				StartTime: time.Date(0, 1, 1, 9, 15, 30, 123, time.UTC),
				EndTime:   time.Date(2000, 1, 1, 17, 45, 0, 0, time.UTC),
			}
			want := time.Date(test.day.Year(), test.day.Month(), test.day.Day(), test.utcHour, 15, 30, 0, time.UTC)
			require.True(t, want.Equal(shift.StartInstant()))
			require.Equal(t, time.Duration(8)*time.Hour+29*time.Minute+30*time.Second, shift.EndInstant().Sub(shift.StartInstant()))
		})
	}
}
