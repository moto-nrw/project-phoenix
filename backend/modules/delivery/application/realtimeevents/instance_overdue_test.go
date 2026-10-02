package realtimeevents

import (
	"errors"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishInstanceOverdueBroadcastsTheBlockTenantWide(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()

	require.NoError(t, PublishInstanceOverdue(publisher, 42, OverdueInstance{
		InstanceID: 151, Date: "2026-04-20", StartTime: "10:00:00", RoomID: 251,
	}))

	calls := publisher.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, "tenant", calls[0].Method)
	assert.Equal(t, int64(42), calls[0].TenantID)
	event := calls[0].Event
	assert.Equal(t, EventInstanceOverdue, event.Type)
	require.NotNil(t, event.Data.InstanceID)
	assert.Equal(t, "151", *event.Data.InstanceID)
	require.NotNil(t, event.Data.InstanceDate)
	assert.Equal(t, "2026-04-20", *event.Data.InstanceDate)
	require.NotNil(t, event.Data.InstanceStartTime)
	assert.Equal(t, "10:00:00", *event.Data.InstanceStartTime)
	require.NotNil(t, event.Data.RoomID)
	assert.Equal(t, "251", *event.Data.RoomID)
}

func TestPublishInstanceOverdueRefreshAsksSupervisionViewsToRefetch(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()

	require.NoError(t, PublishInstanceOverdueRefresh(publisher, 42, 151))

	calls := publisher.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	event := calls[0].Event
	assert.Equal(t, EventActiveSupervisionChanged, event.Type)
	require.NotNil(t, event.Data.InstanceID)
	assert.Equal(t, "151", *event.Data.InstanceID)
	require.NotNil(t, event.Data.Reason)
	assert.Equal(t, "instance_overdue", *event.Data.Reason)
}

func TestPublishInstanceOverdueReturnsTheTransportError(t *testing.T) {
	t.Parallel()
	publisher := testpkg.NewRecordingBroadcaster()
	publisher.Err = errors.New("hub closed")

	assert.ErrorIs(t, PublishInstanceOverdue(publisher, 42, OverdueInstance{InstanceID: 151}), publisher.Err)
	assert.ErrorIs(t, PublishInstanceOverdueRefresh(publisher, 42, 151), publisher.Err)
}
