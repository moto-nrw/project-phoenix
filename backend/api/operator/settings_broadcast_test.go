package operator

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
)

// captureBroadcaster records BroadcastToTenant calls so the helper's
// scheduling behaviour is observable without a live SSE hub. Mirrors
// the captureBroadcaster in api/config — duplicated here to keep the
// operator package self-contained, matching the OnValueSet duplication.
type captureBroadcaster struct {
	tenantID int64
	event    realtime.Event
	called   bool
}

func (c *captureBroadcaster) BroadcastToGroup(_ int64, _ string, _ realtime.Event) error {
	return nil
}

func (c *captureBroadcaster) BroadcastParentMessage(_, _ int64, _ realtime.Event) error { return nil }

func (c *captureBroadcaster) BroadcastToTenant(tenantID int64, event realtime.Event) error {
	c.called = true
	c.tenantID = tenantID
	c.event = event
	return nil
}

func (c *captureBroadcaster) BroadcastToAll(_ realtime.Event) error { return nil }

// TestOperatorScheduleSettingsBroadcast_FiresAfterCommit pins the
// after-commit semantics for the operator-side write path: cross-origin
// tabs MUST NOT be told about the change before the DB commits, otherwise
// they refetch a not-yet-persisted row.
func TestOperatorScheduleSettingsBroadcast_FiresAfterCommit(t *testing.T) {
	bc := &captureBroadcaster{}
	rs := &SettingsResource{broadcaster: bc}

	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())

	rs.scheduleSettingsBroadcast(ctx, int64(7), "operations.session_end_time")

	assert.False(t, bc.called, "broadcast must NOT fire synchronously inside the tx")

	drain()

	assert.True(t, bc.called)
	assert.Equal(t, int64(7), bc.tenantID)
	assert.Equal(t, realtime.EventTenantSettingsChanged, bc.event.Type)
	assert.NotNil(t, bc.event.Data.Source)
	assert.Equal(t, "operations.session_end_time", *bc.event.Data.Source)
}

// TestOperatorScheduleSettingsBroadcast_NoBroadcasterIsNoop covers the
// degraded path where a SettingsResource was constructed without a
// broadcaster (the test setups in api/operator/settings_integration_test.go
// pass nil) — must not blow up.
func TestOperatorScheduleSettingsBroadcast_NoBroadcasterIsNoop(t *testing.T) {
	rs := &SettingsResource{broadcaster: nil}
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())

	assert.NotPanics(t, func() {
		rs.scheduleSettingsBroadcast(ctx, int64(1), "key")
	})
	drain()
}

// TestOperatorScheduleSettingsBroadcast_ZeroTenantIsNoop guards against
// fan-out to a non-tenant context.
func TestOperatorScheduleSettingsBroadcast_ZeroTenantIsNoop(t *testing.T) {
	bc := &captureBroadcaster{}
	rs := &SettingsResource{broadcaster: bc}
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())

	rs.scheduleSettingsBroadcast(ctx, 0, "key")
	drain()

	assert.False(t, bc.called)
}
