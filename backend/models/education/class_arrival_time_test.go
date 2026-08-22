package education_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/models/education"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

func TestClassArrivalTimeValidate(t *testing.T) {
	t.Parallel()

	t.Run("rejects an empty school class", func(t *testing.T) {
		row := &education.ClassArrivalTime{ArrivalTimes: map[string]string{"mon": "11:45"}}
		assert.Error(t, row.Validate())
	})

	t.Run("does not mutate valid input", func(t *testing.T) {
		times := map[string]string{" MON ": " 9:30 ", "tue": "11:45"}
		row := &education.ClassArrivalTime{SchoolClass: "3b", ArrivalTimes: times}

		require.NoError(t, row.Validate())
		assert.Equal(t, times, row.ArrivalTimes)
	})

	t.Run("rejects unknown days, weekend days and malformed times", func(t *testing.T) {
		for name, times := range map[string]map[string]string{
			"unknown day":  {"montag": "11:45"},
			"saturday":     {"sat": "11:45"},
			"sunday":       {"sun": "11:45"},
			"bad time":     {"mon": "11.45"},
			"out of range": {"mon": "25:00"},
		} {
			t.Run(name, func(t *testing.T) {
				row := &education.ClassArrivalTime{SchoolClass: "3b", ArrivalTimes: times}
				assert.Error(t, row.Validate())
			})
		}
	})
}

func TestNormalizeClassArrivalTimes(t *testing.T) {
	t.Parallel()

	normalized, err := education.NormalizeClassArrivalTimes(map[string]string{
		" MON ": " 9:30 ",
		"tue":   "11:45",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"mon": "09:30", "tue": "11:45"}, normalized)

	empty, err := education.NormalizeClassArrivalTimes(map[string]string{"mon": "", "tue": "  "})
	require.NoError(t, err)
	assert.Nil(t, empty)
}

func TestClassArrivalTimeForWeekday(t *testing.T) {
	t.Parallel()

	row := &education.ClassArrivalTime{
		SchoolClass:  "3b",
		ArrivalTimes: map[string]string{"mon": "11:45", "wed": "12:45"},
	}
	require.NoError(t, row.Validate())

	t.Run("returns the wall-clock time for a planned weekday", func(t *testing.T) {
		got, ok := row.TimeForWeekday(scheduleModel.WeekdayMonday)
		require.True(t, ok)
		assert.Equal(t, "11:45", got.Format("15:04"))
	})

	t.Run("reports no time for an unplanned weekday", func(t *testing.T) {
		_, ok := row.TimeForWeekday(scheduleModel.WeekdayTuesday)
		assert.False(t, ok)
	})

	t.Run("a nil row plans nothing", func(t *testing.T) {
		var empty *education.ClassArrivalTime
		_, ok := empty.TimeForWeekday(scheduleModel.WeekdayMonday)
		assert.False(t, ok)
	})
}
