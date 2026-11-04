package schedulerecords

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/require"
)

func TestActivityInstanceValidateExplicitMidnight(t *testing.T) {
	t.Parallel()
	midnight := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	instance := validActivityInstance()
	instance.SetWallClockTimes(midnight, midnight.Add(time.Hour))
	require.NoError(t, instance.Validate())
	instance.RoomID = 0
	require.EqualError(t, instance.Validate(), "room_id is required")
	instance = validActivityInstance()
	instance.StartTime = time.Time{}
	require.EqualError(t, instance.Validate(), "start_time is required")
	instance = validActivityInstance()
	instance.EndTime = time.Time{}
	require.EqualError(t, instance.Validate(), "end_time is required")
}

func TestTimeframeValidateExplicitMidnight(t *testing.T) {
	t.Parallel()
	midnight := calendar.NormalizeWallClock(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	end := time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC)
	timeframe := &Timeframe{EndTime: &end}
	timeframe.SetStartClock(midnight)
	require.NoError(t, timeframe.Validate())
	require.Equal(t, time.Hour, timeframe.Duration(), "optional end clock shares the normalized start anchor")
	require.EqualError(t, (&Timeframe{}).Validate(), "start time is required")
	timeframe.EndTime = &midnight
	require.EqualError(t, timeframe.Validate(), "end time must be after start time")
	require.False(t, timeframe.IsOpen(), "only nil denotes an absent end clock")
}
