package application

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

var lockToday = calendar.NewDate(2026, time.June, 1)

// maxRangeDays mirrors maxRangeDays, which this decision
// test may not import.
const maxRangeDays = 366

func TestSickReportLockRangeSkipsInvalidRanges(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]string{
		{"2026-01-02", "2026-01-01"},
		{"invalid", "2026-01-01"},
	} {
		days := make(map[calendar.Date]bool)
		require.False(t, addDateRange(days, bounds[0], bounds[1], lockToday, true))
		require.Empty(t, days, "invalid ranges must reach Workforce validation without expanding days")
	}
}

func TestSickReportLockRangeIncludesBothBounds(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]string{
		{"2026-01-01", "2027-01-01"}, // 366 days remain whole.
		{"9999-12-30", "9999-12-31"}, // Do not step outside the date format.
	} {
		days := make(map[calendar.Date]bool)
		require.True(t, addDateRange(days, bounds[0], bounds[1], lockToday, true))
		from, err := calendar.ParseDate(bounds[0])
		require.NoError(t, err)
		to, err := calendar.ParseDate(bounds[1])
		require.NoError(t, err)
		require.Len(t, days, from.DaysUntil(to)+1)
		require.True(t, days[from])
		require.True(t, days[to])
	}
}

// Since #6847 every absence type cascades, and a "Sonstige" may run for
// years or end on 9999-12-31. Its lock covers the future days its cascade
// writes, never more than the cap.
func TestSickReportLockRangeBoundsLongAbsencesToTheCascadeWindow(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]string{
		{"2026-01-01", "2027-01-02"},
		{"2020-01-01", "9999-12-31"},
	} {
		days := make(map[calendar.Date]bool)
		require.True(t, addDateRange(days, bounds[0], bounds[1], lockToday, true))
		require.LessOrEqual(t, len(days), maxRangeDays)
		require.True(t, days[lockToday], "the window starts today")
		require.False(t, days[lockToday.AddDays(-1)], "no past day of a long range")
	}
	days := make(map[calendar.Date]bool)
	require.True(t, addDateRange(days, "2020-01-01", "2025-12-31", lockToday, true))
	require.Empty(t, days, "a long range that has ended locks no day")

	days = make(map[calendar.Date]bool)
	require.False(t, addDateRange(days, "2026-01-01", "9999-12-31", lockToday, false))
	require.Empty(t, days, "an oversized sick report reaches validation without a lock")
}

// A sick report is never cut; a long absence cascades over a bounded window
// around today, so its cascade and its lock cover the same future days.
func TestCascadeWindowCutsOnlyLongRanges(t *testing.T) {
	t.Parallel()
	start, end := lockToday.AddDays(-10), lockToday.AddDays(300)
	from, to := cascadeWindow(start, end, lockToday)
	require.Equal(t, start, from)
	require.Equal(t, end, to)

	from, to = cascadeWindow(calendar.NewDate(2020, time.January, 1), calendar.NewDate(9999, time.December, 31), lockToday)
	require.Equal(t, lockToday.AddDays(-maxRangeDays), from)
	require.Equal(t, lockToday.AddDays(maxRangeDays-1), to)

	future := calendar.NewDate(2099, time.January, 6)
	from, to = cascadeWindow(future, calendar.NewDate(2101, time.January, 1), lockToday)
	require.Equal(t, future, from)
	require.Equal(t, future.AddDays(maxRangeDays-1), to)
}
