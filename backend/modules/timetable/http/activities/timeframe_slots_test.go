package httpadapter

import (
	"testing"

	timetableModule "github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
