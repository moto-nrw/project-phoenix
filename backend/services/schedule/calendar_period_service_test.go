package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScheduleErrorUnwrapsSentinel verifies that the ScheduleError wrapper used
// by the calendar period service preserves errors.Is semantics for the
// ErrCalendarPeriodNameConflict sentinel. Callers (API handler) rely on this.
func TestScheduleErrorUnwrapsSentinel(t *testing.T) {
	t.Parallel()

	wrapped := &ScheduleError{Op: "create calendar period", Err: schedule.ErrCalendarPeriodNameConflict}
	assert.True(t, errors.Is(wrapped, schedule.ErrCalendarPeriodNameConflict),
		"ScheduleError must unwrap to the sentinel so handlers can detect via errors.Is")
}

// TestDefaultSchoolYearBounds pins the year-boundary logic of the school-year
// bootstrap (WP-B1). The computation MUST match the frontend helper
// schoolYearPeriodDefaults (timetables/page.tsx): the school year flips on
// August 1st and always spans Aug 1 – Jul 31.
func TestDefaultSchoolYearBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		today     timezone.Date
		wantName  string
		wantStart timezone.Date
		wantEnd   timezone.Date
	}{
		{
			name:      "june belongs to the school year started last August",
			today:     timezone.NewDate(2026, time.June, 12),
			wantName:  "Schuljahr 2025/2026",
			wantStart: timezone.NewDate(2025, time.August, 1),
			wantEnd:   timezone.NewDate(2026, time.July, 31),
		},
		{
			name:      "august 1st starts the new school year",
			today:     timezone.NewDate(2026, time.August, 1),
			wantName:  "Schuljahr 2026/2027",
			wantStart: timezone.NewDate(2026, time.August, 1),
			wantEnd:   timezone.NewDate(2027, time.July, 31),
		},
		{
			name:      "july 31st is still the previous school year",
			today:     timezone.NewDate(2026, time.July, 31),
			wantName:  "Schuljahr 2025/2026",
			wantStart: timezone.NewDate(2025, time.August, 1),
			wantEnd:   timezone.NewDate(2026, time.July, 31),
		},
		{
			name:      "december belongs to the school year started this August",
			today:     timezone.NewDate(2026, time.December, 31),
			wantName:  "Schuljahr 2026/2027",
			wantStart: timezone.NewDate(2026, time.August, 1),
			wantEnd:   timezone.NewDate(2027, time.July, 31),
		},
		{
			name:      "january belongs to the school year started last August",
			today:     timezone.NewDate(2026, time.January, 1),
			wantName:  "Schuljahr 2025/2026",
			wantStart: timezone.NewDate(2025, time.August, 1),
			wantEnd:   timezone.NewDate(2026, time.July, 31),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name, start, end := defaultSchoolYearBounds(tc.today)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
		})
	}
}

// TestFindActiveOverlaps_FastPaths verifies the no-repo-roundtrip branches:
// inactive and nil periods can never produce advisory overlap warnings.
func TestFindActiveOverlaps_FastPaths(t *testing.T) {
	t.Parallel()

	svc := &calendarPeriodService{} // nil repo — must not be touched

	t.Run("nil period returns nil", func(t *testing.T) {
		overlaps, err := svc.FindActiveOverlaps(context.Background(), nil)
		require.NoError(t, err)
		assert.Nil(t, overlaps)
	})

	t.Run("inactive period returns nil without repo call", func(t *testing.T) {
		period := &schedule.CalendarPeriod{
			Name:       "Inaktiv",
			PeriodType: schedule.PeriodTypeSchoolYear,
			StartDate:  timezone.NewDate(2025, time.August, 1),
			EndDate:    timezone.NewDate(2026, time.July, 31),
			IsActive:   false,
		}
		overlaps, err := svc.FindActiveOverlaps(context.Background(), period)
		require.NoError(t, err)
		assert.Nil(t, overlaps)
	})
}

func TestGetUsageCounts_WrapsRepositoryError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("usage query failed")
	svc := &calendarPeriodService{
		repo: usageCountsErrorRepo{
			err: repoErr,
		},
	}

	usage, err := svc.GetUsageCounts(context.Background())

	require.Error(t, err)
	assert.Nil(t, usage)
	assert.ErrorIs(t, err, repoErr)
	assert.Contains(t, err.Error(), "get calendar period usage counts")
}

type usageCountsErrorRepo struct {
	schedule.CalendarPeriodRepository
	err error
}

func (r usageCountsErrorRepo) UsageCounts(context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	return nil, r.err
}

func TestShouldMaterialize(t *testing.T) {
	t.Parallel()

	svc := &calendarPeriodService{}

	// Anchor: Monday 2025-09-01 = "Week A"
	anchor := timezone.NewDate(2025, 9, 1)

	abPeriod := &schedule.CalendarPeriod{
		WeekCycleLength: 2,
		WeekCycleAnchor: &anchor,
	}

	noCyclePeriod := &schedule.CalendarPeriod{
		WeekCycleLength: 1,
	}

	t.Run("week_pattern 0 always materializes", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 8)
		assert.True(t, svc.ShouldMaterialize(0, date, abPeriod))
	})

	t.Run("no cycle period always materializes", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 8)
		assert.True(t, svc.ShouldMaterialize(1, date, noCyclePeriod))
	})

	t.Run("nil period always materializes", func(t *testing.T) {
		date := timezone.NewDate(2025, 9, 8)
		assert.True(t, svc.ShouldMaterialize(1, date, nil))
	})

	t.Run("nil anchor always materializes", func(t *testing.T) {
		p := &schedule.CalendarPeriod{
			WeekCycleLength: 2,
			WeekCycleAnchor: nil,
		}
		date := timezone.NewDate(2025, 9, 8)
		assert.True(t, svc.ShouldMaterialize(1, date, p))
	})

	t.Run("anchor week is week A (pattern 1)", func(t *testing.T) {
		// 2025-09-01 is the anchor itself — should be week A
		date := timezone.NewDate(2025, 9, 1)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("week after anchor is week B (pattern 2)", func(t *testing.T) {
		// 2025-09-08 is 7 days after anchor — should be week B
		date := timezone.NewDate(2025, 9, 8)
		assert.False(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("two weeks after anchor is week A again", func(t *testing.T) {
		// 2025-09-15 is 14 days after anchor — should be week A
		date := timezone.NewDate(2025, 9, 15)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("mid-week day follows week pattern", func(t *testing.T) {
		// 2025-09-03 (Wednesday) is still in the anchor week — week A
		date := timezone.NewDate(2025, 9, 3)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("mid-week day in week B", func(t *testing.T) {
		// 2025-09-10 (Wednesday) is in the second week — week B
		date := timezone.NewDate(2025, 9, 10)
		assert.False(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("date before anchor works correctly", func(t *testing.T) {
		// 2025-08-25 is 7 days BEFORE anchor — should be week B (negative modulo)
		date := timezone.NewDate(2025, 8, 25)
		assert.False(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("date two weeks before anchor is week A", func(t *testing.T) {
		// 2025-08-18 is 14 days before anchor — should be week A
		date := timezone.NewDate(2025, 8, 18)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("year boundary does not break A/B pattern", func(t *testing.T) {
		// This is the critical test. Using day-based math, the pattern should
		// continue correctly across Dec 31 → Jan 1 without ISO week jumps.

		// Anchor is 2025-09-01 (Monday).
		// 2025-12-29 is a Monday.
		// Days diff = (2025-12-29) - (2025-09-01) = 119 days
		// 119 / 7 = 17 weeks
		// 17 % 2 = 1 → pattern = 2 (week B)
		dec29 := timezone.NewDate(2025, 12, 29)
		assert.False(t, svc.ShouldMaterialize(1, dec29, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, dec29, abPeriod))

		// 2026-01-05 is the next Monday (126 days after anchor)
		// 126 / 7 = 18 weeks
		// 18 % 2 = 0 → pattern = 1 (week A)
		jan5 := timezone.NewDate(2026, 1, 5)
		assert.True(t, svc.ShouldMaterialize(1, jan5, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, jan5, abPeriod))
	})

	t.Run("three-week cycle (A/B/C)", func(t *testing.T) {
		threeCycle := &schedule.CalendarPeriod{
			WeekCycleLength: 3,
			WeekCycleAnchor: &anchor,
		}

		// Anchor week = pattern 1
		date := timezone.NewDate(2025, 9, 1)
		assert.True(t, svc.ShouldMaterialize(1, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(2, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(3, date, threeCycle))

		// +1 week = pattern 2
		date = timezone.NewDate(2025, 9, 8)
		assert.False(t, svc.ShouldMaterialize(1, date, threeCycle))
		assert.True(t, svc.ShouldMaterialize(2, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(3, date, threeCycle))

		// +2 weeks = pattern 3
		date = timezone.NewDate(2025, 9, 15)
		assert.False(t, svc.ShouldMaterialize(1, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(2, date, threeCycle))
		assert.True(t, svc.ShouldMaterialize(3, date, threeCycle))

		// +3 weeks = back to pattern 1
		date = timezone.NewDate(2025, 9, 22)
		assert.True(t, svc.ShouldMaterialize(1, date, threeCycle))
	})

	t.Run("large week offset still correct", func(t *testing.T) {
		// 52 weeks (364 days) after anchor
		// 364 / 7 = 52, 52 % 2 = 0 → pattern 1 (week A)
		timezone.NewDate(2026, 8, 31) // ~364 days after 2025-09-01
		// Actually compute exactly: 2025-09-01 + 364 days = 2026-08-31
		exactDate := anchor.AddDays(364)
		assert.True(t, svc.ShouldMaterialize(1, exactDate, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, exactDate, abPeriod))
	})

	t.Run("DST boundary does not break A/B pattern", func(t *testing.T) {
		// Europe/Berlin DST in 2026: CEST begins Sun 2026-03-29 (wall clock
		// 02:00 → 03:00). A week that crosses this boundary is 167h, not 168h.
		// The old float-hours math truncated 167/24 to 6 days, producing the
		// wrong A/B pattern for any anchor on or before the transition.
		dstAnchor := timezone.NewDate(2026, 3, 23)
		dstPeriod := &schedule.CalendarPeriod{
			WeekCycleLength: 2,
			WeekCycleAnchor: &dstAnchor,
		}

		// Mon 2026-03-30 is exactly 7 civil days after the anchor, post-DST.
		// It must resolve to Week B (pattern 2), not Week A.
		mar30UTC := timezone.NewDate(2026, 3, 30)
		assert.False(t, svc.ShouldMaterialize(1, mar30UTC, dstPeriod),
			"post-DST Monday should not be week A")
		assert.True(t, svc.ShouldMaterialize(2, mar30UTC, dstPeriod),
			"post-DST Monday should be week B")

		// Critical: the same civil date expressed in Europe/Berlin local time
		// would, under the old code, subtract to 167h and yield the wrong
		// answer. The fix normalizes both sides to UTC midnight based on the
		// civil date components, which makes the input timezone irrelevant.
		berlin, err := time.LoadLocation("Europe/Berlin")
		require.NoError(t, err)
		mar30Berlin := timezone.DateFromTime(time.Date(2026, 3, 30, 0, 0, 0, 0, berlin))
		assert.False(t, svc.ShouldMaterialize(1, mar30Berlin, dstPeriod),
			"DST-crossing local-time input should still resolve to week B")
		assert.True(t, svc.ShouldMaterialize(2, mar30Berlin, dstPeriod),
			"DST-crossing local-time input should still resolve to week B")

		// And the fall transition (CEST → CET on Sun 2026-10-25 = 169h week).
		// Anchor Mon 2026-10-19 (week A) → Mon 2026-10-26 must be week B.
		fallAnchor := timezone.NewDate(2026, 10, 19)
		fallPeriod := &schedule.CalendarPeriod{
			WeekCycleLength: 2,
			WeekCycleAnchor: &fallAnchor,
		}
		oct26Berlin := timezone.DateFromTime(time.Date(2026, 10, 26, 0, 0, 0, 0, berlin))
		assert.True(t, svc.ShouldMaterialize(2, oct26Berlin, fallPeriod),
			"post-fall-DST Monday should be week B")
	})
}
