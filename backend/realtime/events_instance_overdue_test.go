package realtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInstanceOverdueEventNamesTheBlock(t *testing.T) {
	t.Parallel()

	event := NewInstanceOverdueEvent(151, "2026-04-20", "10:00:00", 251)

	assert.Equal(t, EventInstanceOverdue, event.Type)
	assert.Empty(t, event.ActiveGroupID, "a planned block has no active group to route through")
	require.NotNil(t, event.Data.InstanceID)
	assert.Equal(t, "151", *event.Data.InstanceID)
	require.NotNil(t, event.Data.InstanceDate)
	assert.Equal(t, "2026-04-20", *event.Data.InstanceDate)
	require.NotNil(t, event.Data.InstanceStartTime)
	assert.Equal(t, "10:00:00", *event.Data.InstanceStartTime)
	require.NotNil(t, event.Data.RoomID)
	assert.Equal(t, "251", *event.Data.RoomID)
}

func TestNewInstanceOverdueRefreshEventAsksSupervisionViewsToRefetch(t *testing.T) {
	t.Parallel()

	event := NewInstanceOverdueRefreshEvent(151)

	assert.Equal(t, EventActiveSupervisionChanged, event.Type)
	require.NotNil(t, event.Data.InstanceID)
	assert.Equal(t, "151", *event.Data.InstanceID)
	require.NotNil(t, event.Data.Reason)
	assert.Equal(t, "instance_overdue", *event.Data.Reason)
}
