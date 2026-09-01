package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seriesTestClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return timezone.NormalizeWallClock(parsed)
}

func validSeries(t *testing.T) *StaffShiftSeries {
	t.Helper()
	return &StaffShiftSeries{
		StaffID:          1,
		Weekdays:         []int16{1, 3},
		StartTime:        seriesTestClock(t, "09:00"),
		EndTime:          seriesTestClock(t, "12:00"),
		BreakMinutes:     30,
		CalendarPeriodID: 1,
		WeekPattern:      WeekPatternEvery,
		ValidFrom:        timezone.NewDate(2026, time.September, 1),
		CreatedBy:        1,
	}
}

func TestStaffShiftSeriesValidate(t *testing.T) {
	t.Parallel()

	t.Run("valid series passes", func(t *testing.T) {
		assert.NoError(t, validSeries(t).Validate())
	})

	t.Run("empty weekdays rejected", func(t *testing.T) {
		s := validSeries(t)
		s.Weekdays = nil
		assert.Error(t, s.Validate())
	})

	t.Run("out-of-range weekday rejected", func(t *testing.T) {
		s := validSeries(t)
		s.Weekdays = []int16{0}
		assert.Error(t, s.Validate())
		s.Weekdays = []int16{8}
		assert.Error(t, s.Validate())
	})

	t.Run("duplicate weekday rejected", func(t *testing.T) {
		s := validSeries(t)
		s.Weekdays = []int16{2, 2}
		assert.Error(t, s.Validate())
	})

	t.Run("end before start rejected", func(t *testing.T) {
		s := validSeries(t)
		s.EndTime = seriesTestClock(t, "08:00")
		assert.Error(t, s.Validate())
	})

	t.Run("break over maximum rejected", func(t *testing.T) {
		s := validSeries(t)
		s.StartTime = seriesTestClock(t, "06:00")
		s.EndTime = seriesTestClock(t, "18:00")
		s.BreakMinutes = MaxStaffShiftBreakMinutes + 1
		assert.Error(t, s.Validate())
	})

	t.Run("break longer than shift rejected", func(t *testing.T) {
		s := validSeries(t)
		s.EndTime = seriesTestClock(t, "09:20")
		s.BreakMinutes = 30
		assert.Error(t, s.Validate())
	})

	t.Run("missing calendar period rejected", func(t *testing.T) {
		s := validSeries(t)
		s.CalendarPeriodID = 0
		assert.Error(t, s.Validate())
	})

	t.Run("invalid week pattern rejected", func(t *testing.T) {
		s := validSeries(t)
		s.WeekPattern = 3
		assert.Error(t, s.Validate())
	})

	t.Run("valid_until not after valid_from rejected", func(t *testing.T) {
		s := validSeries(t)
		until := s.ValidFrom
		s.ValidUntil = &until
		assert.Error(t, s.Validate())
	})
}

func TestStaffShiftSeriesRootID(t *testing.T) {
	t.Parallel()

	s := validSeries(t)
	s.ID = 7
	assert.Equal(t, int64(7), s.RootID(), "unsplit series is its own root")
	root := int64(3)
	s.SeriesRootID = &root
	assert.Equal(t, int64(3), s.RootID())
}

func TestStaffShiftSeriesContainsWeekday(t *testing.T) {
	t.Parallel()

	s := validSeries(t)
	assert.True(t, s.ContainsWeekday(1))
	assert.True(t, s.ContainsWeekday(3))
	assert.False(t, s.ContainsWeekday(2))
}
