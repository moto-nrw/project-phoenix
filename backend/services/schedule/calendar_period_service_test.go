package schedule

import (
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/stretchr/testify/assert"
)

// TestScheduleErrorUnwrapsSentinel verifies that the ScheduleError wrapper used
// by the calendar period service preserves errors.Is semantics for the
// ErrCalendarPeriodNameConflict sentinel. Callers (API handler) rely on this.
func TestScheduleErrorUnwrapsSentinel(t *testing.T) {
	wrapped := &ScheduleError{Op: "create calendar period", Err: schedule.ErrCalendarPeriodNameConflict}
	assert.True(t, errors.Is(wrapped, schedule.ErrCalendarPeriodNameConflict),
		"ScheduleError must unwrap to the sentinel so handlers can detect via errors.Is")
}

func TestShouldMaterialize(t *testing.T) {
	svc := &calendarPeriodService{}

	// Anchor: Monday 2025-09-01 = "Week A"
	anchor := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)

	abPeriod := &schedule.CalendarPeriod{
		WeekCycleLength: 2,
		WeekCycleAnchor: &anchor,
	}

	noCyclePeriod := &schedule.CalendarPeriod{
		WeekCycleLength: 1,
	}

	t.Run("week_pattern 0 always materializes", func(t *testing.T) {
		date := time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(0, date, abPeriod))
	})

	t.Run("no cycle period always materializes", func(t *testing.T) {
		date := time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, noCyclePeriod))
	})

	t.Run("nil period always materializes", func(t *testing.T) {
		date := time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, nil))
	})

	t.Run("nil anchor always materializes", func(t *testing.T) {
		p := &schedule.CalendarPeriod{
			WeekCycleLength: 2,
			WeekCycleAnchor: nil,
		}
		date := time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, p))
	})

	t.Run("anchor week is week A (pattern 1)", func(t *testing.T) {
		// 2025-09-01 is the anchor itself — should be week A
		date := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("week after anchor is week B (pattern 2)", func(t *testing.T) {
		// 2025-09-08 is 7 days after anchor — should be week B
		date := time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
		assert.False(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("two weeks after anchor is week A again", func(t *testing.T) {
		// 2025-09-15 is 14 days after anchor — should be week A
		date := time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("mid-week day follows week pattern", func(t *testing.T) {
		// 2025-09-03 (Wednesday) is still in the anchor week — week A
		date := time.Date(2025, 9, 3, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("mid-week day in week B", func(t *testing.T) {
		// 2025-09-10 (Wednesday) is in the second week — week B
		date := time.Date(2025, 9, 10, 0, 0, 0, 0, time.UTC)
		assert.False(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("date before anchor works correctly", func(t *testing.T) {
		// 2025-08-25 is 7 days BEFORE anchor — should be week B (negative modulo)
		date := time.Date(2025, 8, 25, 0, 0, 0, 0, time.UTC)
		assert.False(t, svc.ShouldMaterialize(1, date, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, date, abPeriod))
	})

	t.Run("date two weeks before anchor is week A", func(t *testing.T) {
		// 2025-08-18 is 14 days before anchor — should be week A
		date := time.Date(2025, 8, 18, 0, 0, 0, 0, time.UTC)
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
		dec29 := time.Date(2025, 12, 29, 0, 0, 0, 0, time.UTC)
		assert.False(t, svc.ShouldMaterialize(1, dec29, abPeriod))
		assert.True(t, svc.ShouldMaterialize(2, dec29, abPeriod))

		// 2026-01-05 is the next Monday (126 days after anchor)
		// 126 / 7 = 18 weeks
		// 18 % 2 = 0 → pattern = 1 (week A)
		jan5 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, jan5, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, jan5, abPeriod))
	})

	t.Run("three-week cycle (A/B/C)", func(t *testing.T) {
		threeCycle := &schedule.CalendarPeriod{
			WeekCycleLength: 3,
			WeekCycleAnchor: &anchor,
		}

		// Anchor week = pattern 1
		date := time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(2, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(3, date, threeCycle))

		// +1 week = pattern 2
		date = time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)
		assert.False(t, svc.ShouldMaterialize(1, date, threeCycle))
		assert.True(t, svc.ShouldMaterialize(2, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(3, date, threeCycle))

		// +2 weeks = pattern 3
		date = time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC)
		assert.False(t, svc.ShouldMaterialize(1, date, threeCycle))
		assert.False(t, svc.ShouldMaterialize(2, date, threeCycle))
		assert.True(t, svc.ShouldMaterialize(3, date, threeCycle))

		// +3 weeks = back to pattern 1
		date = time.Date(2025, 9, 22, 0, 0, 0, 0, time.UTC)
		assert.True(t, svc.ShouldMaterialize(1, date, threeCycle))
	})

	t.Run("large week offset still correct", func(t *testing.T) {
		// 52 weeks (364 days) after anchor
		// 364 / 7 = 52, 52 % 2 = 0 → pattern 1 (week A)
		date := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC) // ~364 days after 2025-09-01
		// Actually compute exactly: 2025-09-01 + 364 days = 2026-08-31
		exactDate := anchor.AddDate(0, 0, 364)
		assert.True(t, svc.ShouldMaterialize(1, exactDate, abPeriod))
		assert.False(t, svc.ShouldMaterialize(2, exactDate, abPeriod))
		_ = date // used for documentation
	})
}
