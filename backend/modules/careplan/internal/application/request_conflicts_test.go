package application

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConflictWeekdayUsesOwnerKey(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"", "pickup:2026-09-20", "care:0:pickup", "care:8:pickup", "care:mon:pickup", "care:1:pickup:extra"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			_, err := conflictKeyWeekday(key)
			require.ErrorIs(t, err, carerequests.ErrStaffValueUnsupported)
		})
	}
	for _, tc := range []struct {
		key     string
		weekday int
	}{{"care:1:pickup", 1}, {"care:7:pickup", 7}} {
		weekday, err := conflictKeyWeekday(tc.key)
		require.NoError(t, err)
		assert.Equal(t, tc.weekday, weekday)
	}
}

func TestConflictStaffPickupTimeNormalizesClock(t *testing.T) {
	t.Parallel()
	pickup, err := staffPickupTime(" 07:30 ")
	require.NoError(t, err)
	assert.Equal(t, "07:30", pickup.Format("15:04"))
	assert.Equal(t, 1, pickup.Year())
	midnight, err := staffPickupTime("00:00")
	require.NoError(t, err)
	assert.True(t, midnight.IsZero())
	for _, value := range []string{"", "25:00", "bad"} {
		_, err := staffPickupTime(value)
		require.ErrorIs(t, err, carerequests.ErrInvalidPayload)
	}
}
