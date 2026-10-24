package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type parentEventCapture struct {
	tenantID, studentID, guardianID int64
	event                           communication.ParentChildEvent
	wakeTenant, wakeStudent         int64
}

func (c *parentEventCapture) EmitParentChildEvent(_ context.Context, tenantID, studentID, guardianID int64, event communication.ParentChildEvent) error {
	c.tenantID, c.studentID, c.guardianID, c.event = tenantID, studentID, guardianID, event
	return nil
}

func (c *parentEventCapture) BroadcastChildUpdateToGuardians(tenantID, studentID int64) {
	c.wakeTenant, c.wakeStudent = tenantID, studentID
}

func TestParentChildEventsKeepRecipientAndActorSeparate(t *testing.T) {
	t.Parallel()

	owner := &parentEventCapture{}
	bound := BindParentChildEvents(owner)
	refID := int64(57)
	_ = bound.EmitChildEvent(context.Background(), 11, 23, 31, parentChildEvent{
		EventType: "request_created", ActorKind: "guardian", ActorAccountID: 43,
		Body: "request", RequestType: "master_data", RequestStatus: "offen",
		RefTable: "users.student_data_change_requests", RefID: &refID,
	})
	assert.Equal(t, int64(11), owner.tenantID)
	assert.Equal(t, int64(23), owner.studentID)
	assert.Equal(t, int64(31), owner.guardianID)
	assert.Equal(t, communication.ParentChildEvent{
		EventType: "request_created", ActorKind: "guardian", ActorAccountID: 43,
		Body: "request", RequestType: "master_data", RequestStatus: "offen",
		RefTable: "users.student_data_change_requests", RefID: &refID,
	}, owner.event)
	bound.BroadcastChildUpdateToGuardians(61, 73)
	assert.Equal(t, int64(61), owner.wakeTenant)
	assert.Equal(t, int64(73), owner.wakeStudent)
	assert.Nil(t, BindParentChildEvents(nil))
}

type parentMessagingFlag struct {
	t        *testing.T
	ctx      context.Context
	tenantID int64
	enabled  bool
	err      error
}

func (f parentMessagingFlag) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	f.t.Helper()
	require.Same(f.t, f.ctx, ctx)
	require.Equal(f.t, f.tenantID, tenantID)
	require.Equal(f.t, "operations.parent_notes_enabled", key)
	return f.enabled, f.err
}

func TestParentMessagingGatePreservesTenantAndFailureDirection(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, test := range []struct {
		name    string
		enabled bool
		err     error
		want    bool
	}{
		{name: "enabled", enabled: true, want: true},
		{name: "disabled", want: false},
		{name: "resolution failure", err: errors.New("unavailable"), want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gate := BindParentMessagingGate(parentMessagingFlag{
				t: t, ctx: ctx, tenantID: 83, enabled: test.enabled, err: test.err,
			}, nil)
			assert.Equal(t, test.want, gate(ctx, 83))
		})
	}
}
