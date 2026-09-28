package timetable_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func coverageClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return calendar.NormalizeWallClock(parsed)
}

func coverageWindow(t *testing.T, start, end string) timetable.ShiftWindow {
	t.Helper()
	return timetable.ShiftWindow{StartTime: coverageClock(t, start), EndTime: coverageClock(t, end)}
}

func formattedGaps(gaps []timetable.ShiftCoverageInterval) [][2]string {
	out := make([][2]string, 0, len(gaps))
	for _, gap := range gaps {
		out = append(out, [2]string{
			calendar.NormalizeWallClock(gap.StartTime).Format("15:04"),
			calendar.NormalizeWallClock(gap.EndTime).Format("15:04"),
		})
	}
	return out
}

func TestUncoveredShiftIntervals(t *testing.T) {
	t.Parallel()

	start := coverageClock(t, "08:00")
	end := coverageClock(t, "12:00")
	cancelled := coverageWindow(t, "07:00", "13:00")
	cancelled.Cancelled = true

	tests := []struct {
		name   string
		shifts []timetable.ShiftWindow
		want   [][2]string
	}{
		{name: "one shift covers", shifts: []timetable.ShiftWindow{coverageWindow(t, "07:00", "13:00")}, want: [][2]string{}},
		{name: "no shift", want: [][2]string{{"08:00", "12:00"}}},
		{name: "starts too late", shifts: []timetable.ShiftWindow{coverageWindow(t, "09:00", "13:00")}, want: [][2]string{{"08:00", "09:00"}}},
		{name: "ends too early", shifts: []timetable.ShiftWindow{coverageWindow(t, "07:00", "11:00")}, want: [][2]string{{"11:00", "12:00"}}},
		{name: "touching shifts cover", shifts: []timetable.ShiftWindow{
			coverageWindow(t, "08:00", "10:00"),
			coverageWindow(t, "10:00", "12:00"),
		}, want: [][2]string{}},
		{name: "gap is exact", shifts: []timetable.ShiftWindow{
			coverageWindow(t, "10:00", "12:00"),
			coverageWindow(t, "07:00", "09:00"),
		}, want: [][2]string{{"09:00", "10:00"}}},
		{name: "overlaps merge", shifts: []timetable.ShiftWindow{
			coverageWindow(t, "07:00", "10:30"),
			coverageWindow(t, "09:00", "13:00"),
		}, want: [][2]string{}},
		{name: "cancelled shift covers nothing", shifts: []timetable.ShiftWindow{cancelled}, want: [][2]string{{"08:00", "12:00"}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, formattedGaps(timetable.UncoveredShiftIntervals(start, end, tc.shifts)))
		})
	}
}

func TestContainingCalendarWeek(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.July, 6)
	for offset := range 7 {
		from, to := timetable.ContainingCalendarWeek(monday.AddDays(offset))
		assert.Equal(t, monday, from)
		assert.Equal(t, monday.AddDays(6), to)
	}
}

func TestIndexCalendarWeeks_SkipsZeroWeeks(t *testing.T) {
	t.Parallel()

	monday := calendar.NewDate(2026, time.July, 6)
	assert.Equal(t, map[calendar.Date]bool{monday: true}, timetable.IndexCalendarWeeks([]calendar.Date{monday, "", monday}))
}
