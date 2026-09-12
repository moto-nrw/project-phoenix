package excusedrequests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReviewCalendarArithmeticUsesDaysAcrossDSTAndLeapYears(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		day    string
		offset int
		want   Date
	}{
		{"2026-03-28", 2, "2026-03-30"},
		{"2026-10-24", 2, "2026-10-26"},
		{"2028-02-28", 1, "2028-02-29"},
		{"2028-03-01", -1, "2028-02-29"},
	} {
		day, err := ParseDate(tc.day)
		require.NoError(t, err)
		require.Equal(t, tc.want, day.AddDays(tc.offset))
	}
	require.Equal(t, Date("2026-09-07"), Date("2026-09-13").StartOfISOWeek())
	require.Equal(t, "13.09.2026", Date("2026-09-13").Format("02.01.2006"))
	for _, invalid := range []string{"", "2026-02-29", "2026-2-01", "2026-01-01T00:00:00Z"} {
		_, err := ParseDate(invalid)
		require.Error(t, err)
	}
}
