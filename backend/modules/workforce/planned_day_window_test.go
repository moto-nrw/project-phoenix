package workforce_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func TestPlannedDayWindow(t *testing.T) {
	t.Parallel()
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	at := func(hour int) time.Time { return time.Date(2026, time.July, 6, hour, 0, 0, 0, berlin) }

	t.Run("no shift is unplanned", func(t *testing.T) {
		t.Parallel()
		_, _, ok := workforce.PlannedDayWindow(nil)
		assert.False(t, ok)
	})

	t.Run("only cancelled shifts are unplanned", func(t *testing.T) {
		t.Parallel()
		_, _, ok := workforce.PlannedDayWindow([]workforce.PlannedShiftSpan{{Start: at(8), End: at(12), Cancelled: true}})
		assert.False(t, ok)
	})

	t.Run("earliest start and latest end of the shifts that take place", func(t *testing.T) {
		t.Parallel()
		start, end, ok := workforce.PlannedDayWindow([]workforce.PlannedShiftSpan{
			{Start: at(13), End: at(15)},
			{Start: at(7), End: at(18), Cancelled: true},
			{Start: at(9), End: at(12)},
		})
		assert.True(t, ok)
		assert.Equal(t, at(9), start)
		assert.Equal(t, at(15), end)
	})
}

func TestCheckInOpensAt(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, time.July, 6, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, start.Add(-5*time.Minute), workforce.CheckInOpensAt(start, 5))
	assert.Equal(t, start, workforce.CheckInOpensAt(start, 0))
}
