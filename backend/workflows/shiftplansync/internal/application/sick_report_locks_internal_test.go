package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

func TestSickReportLockRangeSkipsInvalidRanges(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]string{
		{"2026-01-01", "2027-01-02"},
		{"2026-01-01", "9999-12-31"},
		{"2026-01-02", "2026-01-01"},
		{"invalid", "2026-01-01"},
	} {
		days := make(map[calendar.Date]bool)
		addDateRange(days, bounds[0], bounds[1])
		require.Empty(t, days, "invalid ranges must reach Workforce validation without expanding days")
	}
}

func TestSickReportLockRangeIncludesBothBounds(t *testing.T) {
	t.Parallel()
	for _, bounds := range [][2]string{
		{"2026-01-01", "2027-01-01"}, // 366 days remain allowed.
		{"9999-12-30", "9999-12-31"}, // Do not step outside the date format.
	} {
		days := make(map[calendar.Date]bool)
		require.True(t, addDateRange(days, bounds[0], bounds[1]))
		from, err := calendar.ParseDate(bounds[0])
		require.NoError(t, err)
		to, err := calendar.ParseDate(bounds[1])
		require.NoError(t, err)
		require.Len(t, days, from.DaysUntil(to)+1)
		require.True(t, days[from])
		require.True(t, days[to])
	}
}
