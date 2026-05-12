package sessions

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

type captureMirrorBroadcaster struct {
	called   bool
	tenantID int64
	event    realtime.Event
}

func (b *captureMirrorBroadcaster) BroadcastToGroup(_ int64, _ string, _ realtime.Event) error {
	return nil
}

func (b *captureMirrorBroadcaster) BroadcastToTenant(tenantID int64, event realtime.Event) error {
	b.called = true
	b.tenantID = tenantID
	b.event = event
	return nil
}

func (b *captureMirrorBroadcaster) BroadcastToAll(_ realtime.Event) error { return nil }

func TestBroadcastMirroredInstanceFiresAfterCommit(t *testing.T) {
	bc := &captureMirrorBroadcaster{}
	rs := &Resource{Broadcaster: bc}
	ctx, drain := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 42))
	activeGroupID := int64(321)
	inst := &scheduleModel.ActivityInstance{
		Date:          time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
		StartTime:     time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		ActiveGroupID: &activeGroupID,
	}
	inst.ID = 123

	rs.broadcastMirroredInstance(ctx, realtime.EventInstanceStarted, inst, &active.Group{RoomID: 77})

	assert.False(t, bc.called, "mirrored timetable SSE must wait for commit")

	drain()

	assert.True(t, bc.called)
	assert.Equal(t, int64(42), bc.tenantID)
	assert.Equal(t, realtime.EventInstanceStarted, bc.event.Type)
	assert.Equal(t, "321", bc.event.ActiveGroupID)
	requireData := bc.event.Data
	assert.NotNil(t, requireData.InstanceID)
	assert.Equal(t, "123", *requireData.InstanceID)
}
