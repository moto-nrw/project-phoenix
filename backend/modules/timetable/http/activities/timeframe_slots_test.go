package httpadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTimeframes answers the overlap listing with fixed timeframes and
// records the clock window it was asked for.
type fakeTimeframes struct {
	timeframes []timetableModule.Timeframe
	err        error
	from, to   string
}

func (f *fakeTimeframes) FindTimeframe(context.Context, int64) (timetableModule.Timeframe, error) {
	panic("unused")
}

func (f *fakeTimeframes) ListTimeframes(_ context.Context, filter timetableModule.TimeframeFilter) ([]timetableModule.Timeframe, error) {
	if filter.OverlapsStart != nil {
		f.from = *filter.OverlapsStart
	}
	if filter.OverlapsEnd != nil {
		f.to = *filter.OverlapsEnd
	}
	return f.timeframes, f.err
}

func clockTimeframe(id int64, start string, end *string) timetableModule.Timeframe {
	return timetableModule.Timeframe{ID: id, StartTime: start, EndTime: end, IsActive: true}
}

func ptr(value string) *string { return &value }

func TestTimeframeSlotsReadTheOwnerClocks(t *testing.T) {
	t.Parallel()

	morning := clockTimeframe(4, "08:00:00", ptr("09:30:00"))
	slot, err := timeframeToSlot(morning)
	require.NoError(t, err)
	assert.Equal(t, morning.ID, slot.ID)
	assert.Equal(t, "08:00", slot.StartTime.Format("15:04"))
	require.NotNil(t, slot.EndTime)
	assert.Equal(t, "09:30", slot.EndTime.Format("15:04"))

	open, err := timeframeToSlot(clockTimeframe(5, "13:00:00", nil))
	require.NoError(t, err)
	assert.Nil(t, open.EndTime)

	_, err = timeframeToSlot(clockTimeframe(6, "not a clock", nil))
	require.Error(t, err)
}

func TestAvailableTimeframeSlots(t *testing.T) {
	t.Parallel()

	base := time.Date(2000, 1, 1, 8, 0, 0, 0, time.UTC)

	t.Run("finds the gaps between timeframes", func(t *testing.T) {
		// 08:00-09:00 and 11:00-12:00 are taken; searching 07:00-13:00 for
		// one-hour slots yields 07:00-08:00, 09:00-11:00 and 12:00-13:00.
		source := &fakeTimeframes{timeframes: []timetableModule.Timeframe{
			clockTimeframe(2, "11:00:00", ptr("12:00:00")),
			clockTimeframe(1, "08:00:00", ptr("09:00:00")),
		}}
		slots, err := availableTimeframeSlots(context.Background(), source, base.Add(-time.Hour), base.Add(5*time.Hour), time.Hour)
		require.NoError(t, err)
		require.Len(t, slots, 3)
		assert.Equal(t, "07:00", slots[0].StartTime.Format("15:04"))
		assert.Equal(t, "08:00", slots[0].EndTime.Format("15:04"))
		assert.Equal(t, "09:00", slots[1].StartTime.Format("15:04"))
		assert.Equal(t, "11:00", slots[1].EndTime.Format("15:04"))
		assert.Equal(t, "12:00", slots[2].StartTime.Format("15:04"))
		assert.Equal(t, "13:00", slots[2].EndTime.Format("15:04"))
		for _, slot := range slots {
			assert.True(t, slot.IsActive)
		}
	})

	t.Run("drops gaps shorter than the duration", func(t *testing.T) {
		source := &fakeTimeframes{timeframes: []timetableModule.Timeframe{clockTimeframe(1, "08:00:00", ptr("18:00:00"))}}
		slots, err := availableTimeframeSlots(context.Background(), source, base, base.Add(10*time.Hour), 5*time.Hour)
		require.NoError(t, err)
		assert.Empty(t, slots)
	})

	t.Run("an open-ended timeframe closes the search", func(t *testing.T) {
		source := &fakeTimeframes{timeframes: []timetableModule.Timeframe{clockTimeframe(1, "09:00:00", nil)}}
		slots, err := availableTimeframeSlots(context.Background(), source, base, base.Add(6*time.Hour), 30*time.Minute)
		require.NoError(t, err)
		require.Len(t, slots, 1, "only the gap before the open-ended timeframe is offered")
		assert.Equal(t, "08:00", slots[0].StartTime.Format("15:04"))
	})

	t.Run("rejects an inverted date range and a zero duration", func(t *testing.T) {
		_, err := availableTimeframeSlots(context.Background(), &fakeTimeframes{}, base.Add(time.Hour), base, time.Hour)
		require.ErrorIs(t, err, timetableModule.ErrInvalidRecurrenceRange)
		_, err = availableTimeframeSlots(context.Background(), &fakeTimeframes{}, base, base.Add(time.Hour), 0)
		require.ErrorIs(t, err, timetableModule.ErrInvalidDuration)
	})

	t.Run("propagates the owner error", func(t *testing.T) {
		_, err := availableTimeframeSlots(context.Background(), &fakeTimeframes{err: errors.New("boom")}, base, base.Add(time.Hour), time.Hour)
		require.Error(t, err)
	})
}
