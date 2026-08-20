package config

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

// captureBroadcaster records BroadcastToTenant calls so the helper's
// scheduling behaviour is observable without a live SSE hub.
type captureBroadcaster struct {
	tenantID int64
	event    realtime.Event
	called   bool
}

func (c *captureBroadcaster) BroadcastToGroup(_ int64, _ string, _ realtime.Event) error {
	return nil
}

func (c *captureBroadcaster) BroadcastParentMessage(_, _ int64, _ realtime.Event) error { return nil }
func (c *captureBroadcaster) BroadcastToGuardian(_, _ int64, _ realtime.Event) error    { return nil }

func (c *captureBroadcaster) BroadcastToTenant(tenantID int64, event realtime.Event) error {
	c.called = true
	c.tenantID = tenantID
	c.event = event
	return nil
}

func (c *captureBroadcaster) BroadcastToTenantAdmins(_ int64, _ realtime.Event) error { return nil }

func (c *captureBroadcaster) BroadcastToStaffAccounts(_ int64, _ []int64, _ realtime.Event) error {
	return nil
}

func (c *captureBroadcaster) BroadcastToAll(_ realtime.Event) error { return nil }

// TestScheduleSettingsBroadcast_FiresAfterCommit pins the contract: the
// broadcast does NOT happen synchronously inside the in-tx callback —
// it's queued via RegisterAfterCommit and only fires when the holder is
// drained (i.e. after a successful commit). This is what protects the
// frontend from refetching against a row that didn't actually persist.
func TestScheduleSettingsBroadcast_FiresAfterCommit(t *testing.T) {
	t.Parallel()

	bc := &captureBroadcaster{}
	rs := &SettingsResource{broadcaster: bc}

	// Hand-craft a tx-style ctx that owns an after-commit hooks holder so
	// scheduleSettingsBroadcast queues into it instead of running inline.
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())

	rs.scheduleSettingsBroadcast(ctx, int64(42), "operations.session_end_time")

	assert.False(t, bc.called, "broadcast must NOT fire synchronously inside the tx")

	drain()

	assert.True(t, bc.called, "broadcast must fire when the after-commit holder drains")
	assert.Equal(t, int64(42), bc.tenantID)
	assert.Equal(t, realtime.EventTenantSettingsChanged, bc.event.Type)
	assert.NotNil(t, bc.event.Data.Source)
	assert.Equal(t, "operations.session_end_time", *bc.event.Data.Source)
}

// TestScheduleSettingsBroadcast_NoBroadcasterIsNoop covers the happy
// degraded path: SettingsResource constructed without a broadcaster (the
// test setups in api/config/settings_integration_test.go pass nil)
// must not blow up — it just skips the broadcast.
func TestScheduleSettingsBroadcast_NoBroadcasterIsNoop(t *testing.T) {
	t.Parallel()

	rs := &SettingsResource{broadcaster: nil}
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())

	assert.NotPanics(t, func() {
		rs.scheduleSettingsBroadcast(ctx, int64(1), "key")
	})
	drain()
}

// TestScheduleSettingsBroadcast_ZeroTenantIsNoop guards against
// accidentally fanning out to a non-tenant context (tenant ID 0 indicates
// a broken caller — better to swallow than to broadcast to "everyone").
func TestScheduleSettingsBroadcast_ZeroTenantIsNoop(t *testing.T) {
	t.Parallel()

	bc := &captureBroadcaster{}
	rs := &SettingsResource{broadcaster: bc}
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())

	rs.scheduleSettingsBroadcast(ctx, 0, "key")
	drain()

	assert.False(t, bc.called)
}
