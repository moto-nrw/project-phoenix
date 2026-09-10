package services

import (
	"log/slog"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKioskMirrorPublishesOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	bc := testpkg.NewRecordingBroadcaster()
	ctx, drain := testpkg.WithAfterCommitHooks(testpkg.TenantContext(42))
	groupID := int64(321)
	publish := KioskMirrorPublisher(bc, slog.Default())
	publish(ctx, KioskMirroredSession{ID: 123, Date: "2026-05-11", StartTime: "14:00:00", RoomID: 77, ActiveGroupID: &groupID})
	assert.Empty(t, bc.CallsByMethod("tenant"))
	drain()
	calls := bc.CallsByMethod("tenant")
	require.Len(t, calls, 1)
	assert.Equal(t, int64(42), calls[0].TenantID)
	assert.Equal(t, "instance_started", string(calls[0].Event.Type))
	assert.Equal(t, "321", calls[0].Event.ActiveGroupID)
	assert.Equal(t, "123", *calls[0].Event.Data.InstanceID)
	assert.Equal(t, "77", *calls[0].Event.Data.RoomID)
	assert.Equal(t, "2026-05-11", *calls[0].Event.Data.InstanceDate)
	assert.Equal(t, "14:00:00", *calls[0].Event.Data.InstanceStartTime)
}
