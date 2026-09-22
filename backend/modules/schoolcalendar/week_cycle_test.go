package schoolcalendar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultSchoolYear pins the year-boundary logic of the school-year
// bootstrap (WP-B1). The computation MUST match the frontend helper
// schoolYearPeriodDefaults (timetables/page.tsx): the school year flips on
// August 1st and always spans Aug 1 – Jul 31.
func TestDefaultSchoolYear(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		today     string
		wantName  string
		wantStart string
		wantEnd   string
	}{
		{"june belongs to the school year started last August", "2026-06-12", "Schuljahr 2025/2026", "2025-08-01", "2026-07-31"},
		{"august 1st starts the new school year", "2026-08-01", "Schuljahr 2026/2027", "2026-08-01", "2027-07-31"},
		{"july 31st is still the previous school year", "2026-07-31", "Schuljahr 2025/2026", "2025-08-01", "2026-07-31"},
		{"december belongs to the school year started this August", "2026-12-31", "Schuljahr 2026/2027", "2026-08-01", "2027-07-31"},
		{"january belongs to the school year started last August", "2026-01-01", "Schuljahr 2025/2026", "2025-08-01", "2026-07-31"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, start, end := DefaultSchoolYear(tc.today)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
		})
	}

	name, start, end := DefaultSchoolYear("not a date")
	assert.Empty(t, name)
	assert.Empty(t, start)
	assert.Empty(t, end)
}

func TestWeekPatternApplies(t *testing.T) {
	t.Parallel()

	// Anchor: Monday 2025-09-01 = "Week A"
	abCycle := WeekCycle{Length: 2, Anchor: "2025-09-01"}
	noCycle := WeekCycle{Length: 1}

	t.Run("week_pattern 0 always applies", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(0, "2025-09-08", abCycle))
	})

	t.Run("no cycle always applies", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(1, "2025-09-08", noCycle))
	})

	t.Run("zero period always applies", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(1, "2025-09-08", CalendarPeriod{}.WeekCycle()))
	})

	t.Run("no anchor always applies", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(1, "2025-09-08", WeekCycle{Length: 2}))
	})

	t.Run("unparseable date applies rather than silently dropping", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(2, "nope", abCycle))
		assert.True(t, WeekPatternApplies(2, "2025-09-01", WeekCycle{Length: 2, Anchor: "nope"}))
	})

	t.Run("anchor week is week A (pattern 1)", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(1, "2025-09-01", abCycle))
		assert.False(t, WeekPatternApplies(2, "2025-09-01", abCycle))
	})

	t.Run("week after anchor is week B (pattern 2)", func(t *testing.T) {
		assert.False(t, WeekPatternApplies(1, "2025-09-08", abCycle))
		assert.True(t, WeekPatternApplies(2, "2025-09-08", abCycle))
	})

	t.Run("two weeks after anchor is week A again", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(1, "2025-09-15", abCycle))
		assert.False(t, WeekPatternApplies(2, "2025-09-15", abCycle))
	})

	t.Run("mid-week day follows week pattern", func(t *testing.T) {
		assert.True(t, WeekPatternApplies(1, "2025-09-03", abCycle))
		assert.False(t, WeekPatternApplies(2, "2025-09-03", abCycle))
		assert.False(t, WeekPatternApplies(1, "2025-09-10", abCycle))
		assert.True(t, WeekPatternApplies(2, "2025-09-10", abCycle))
	})

	t.Run("dates before the anchor use the floored week index", func(t *testing.T) {
		// 7 days BEFORE the anchor is week B (negative modulo).
		assert.False(t, WeekPatternApplies(1, "2025-08-25", abCycle))
		assert.True(t, WeekPatternApplies(2, "2025-08-25", abCycle))
		// 14 days before is week A again.
		assert.True(t, WeekPatternApplies(1, "2025-08-18", abCycle))
		assert.False(t, WeekPatternApplies(2, "2025-08-18", abCycle))
		// A partial week before the anchor still belongs to week B.
		assert.True(t, WeekPatternApplies(2, "2025-08-29", abCycle))
	})

	t.Run("year boundary does not break A/B pattern", func(t *testing.T) {
		// 2025-12-29 is 119 days after the anchor: 17 weeks, 17 % 2 = 1 → week B.
		assert.False(t, WeekPatternApplies(1, "2025-12-29", abCycle))
		assert.True(t, WeekPatternApplies(2, "2025-12-29", abCycle))
		// 2026-01-05 is 126 days after: 18 weeks → week A.
		assert.True(t, WeekPatternApplies(1, "2026-01-05", abCycle))
		assert.False(t, WeekPatternApplies(2, "2026-01-05", abCycle))
	})

	t.Run("three-week cycle (A/B/C)", func(t *testing.T) {
		threeCycle := WeekCycle{Length: 3, Anchor: "2025-09-01"}
		assert.True(t, WeekPatternApplies(1, "2025-09-01", threeCycle))
		assert.False(t, WeekPatternApplies(2, "2025-09-01", threeCycle))
		assert.False(t, WeekPatternApplies(3, "2025-09-01", threeCycle))
		assert.True(t, WeekPatternApplies(2, "2025-09-08", threeCycle))
		assert.True(t, WeekPatternApplies(3, "2025-09-15", threeCycle))
		assert.True(t, WeekPatternApplies(1, "2025-09-22", threeCycle))
	})

	t.Run("large week offset still correct", func(t *testing.T) {
		// 364 days after the anchor: 52 weeks → week A.
		assert.True(t, WeekPatternApplies(1, "2026-08-31", abCycle))
		assert.False(t, WeekPatternApplies(2, "2026-08-31", abCycle))
	})

	t.Run("DST boundary does not break A/B pattern", func(t *testing.T) {
		// Europe/Berlin DST in 2026: CEST begins Sun 2026-03-29. A week that
		// crosses this boundary is 167h, not 168h; the day-based math must
		// still put Mon 2026-03-30 into week B.
		dstCycle := WeekCycle{Length: 2, Anchor: "2026-03-23"}
		assert.False(t, WeekPatternApplies(1, "2026-03-30", dstCycle), "post-DST Monday should not be week A")
		assert.True(t, WeekPatternApplies(2, "2026-03-30", dstCycle), "post-DST Monday should be week B")

		berlin, err := time.LoadLocation("Europe/Berlin")
		require.NoError(t, err)
		mar30Berlin := time.Date(2026, 3, 30, 0, 0, 0, 0, berlin).Format(DateLayout)
		assert.True(t, WeekPatternApplies(2, mar30Berlin, dstCycle), "the civil date decides, not the instant")

		// And the fall transition (CEST → CET on Sun 2026-10-25 = 169h week).
		fallCycle := WeekCycle{Length: 2, Anchor: "2026-10-19"}
		assert.True(t, WeekPatternApplies(2, "2026-10-26", fallCycle), "post-fall-DST Monday should be week B")
	})
}
