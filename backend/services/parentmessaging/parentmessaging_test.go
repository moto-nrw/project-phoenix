package parentmessaging

// Unit tests for the consumer-facing shell: the fail-OPEN feature gate, the
// terminal-event rule, and the guardian access check over the Events port.
// Everything that touches persistence or realtime is Communication's and is
// tested with its owner.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes --------------------------------------------------------------

type fakeSettings struct {
	val bool
	err error
}

func (f fakeSettings) ResolveBool(context.Context, string) (bool, error) { return f.val, f.err }

type fakeTenantSettings struct {
	val bool
	err error
}

func (f fakeTenantSettings) ResolveBoolForTenant(context.Context, int64, string) (bool, error) {
	return f.val, f.err
}

type fakeEvents struct {
	Events
	guardians    []PortalGuardian
	guardiansErr error
	emitted      []ChildEvent
	woken        []int64
}

func (f *fakeEvents) PortalGuardians(context.Context, int64) ([]PortalGuardian, error) {
	return f.guardians, f.guardiansErr
}

func (f *fakeEvents) EmitChildEvent(_, _, _ int64, ev ChildEvent) { f.emitted = append(f.emitted, ev) }

func (f *fakeEvents) WakeChildGuardians(_, studentID int64) { f.woken = append(f.woken, studentID) }

// --- MessagingEnabled ---------------------------------------------------

func TestMessagingEnabled(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.True(t, MessagingEnabled(ctx, fakeSettings{val: true}, nil))
	assert.False(t, MessagingEnabled(ctx, fakeSettings{val: false}, nil))
	// Fail OPEN: a transient resolve error counts as enabled so the read and write
	// paths can't disagree during a config-DB blip.
	assert.True(t, MessagingEnabled(ctx, fakeSettings{err: errors.New("settings down")}, nil),
		"a resolve error must fail open (enabled)")
}

func TestMessagingEnabledForTenant(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	assert.True(t, MessagingEnabledForTenant(ctx, fakeTenantSettings{val: true}, 5, nil))
	assert.False(t, MessagingEnabledForTenant(ctx, fakeTenantSettings{val: false}, 5, nil))
	assert.True(t, MessagingEnabledForTenant(ctx, fakeTenantSettings{err: errors.New("down")}, 5, nil),
		"a per-tenant resolve error must fail open")
}

// --- GuardianHasChildAccess ---------------------------------------------

// TestGuardianHasChildAccess pins the linked-guardian / parent_portal.access
// gate shared by the emitter's staff-decision-pill suppression and the
// care-schedule request approve gate: an account still present in the
// portal-guardian list has access; an unlinked/revoked account (absent from
// the list) does not; a port error propagates; and an unwired emitter returns
// a configuration error rather than silently granting access.
func TestGuardianHasChildAccess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	linked := NewEmitter(&fakeEvents{guardians: []PortalGuardian{{AccountID: 7}, {AccountID: 9}}})
	ok, err := linked.GuardianHasChildAccess(ctx, 1, 7)
	require.NoError(t, err)
	assert.True(t, ok, "an account in the portal-guardian list has access")

	ok, err = linked.GuardianHasChildAccess(ctx, 1, 42)
	require.NoError(t, err)
	assert.False(t, ok, "a revoked/unlinked account is absent from the list → no access")

	failing := NewEmitter(&fakeEvents{guardiansErr: errors.New("db down")})
	_, err = failing.GuardianHasChildAccess(ctx, 1, 7)
	require.Error(t, err, "a port error must propagate, not be read as revoked")

	var unwired *Emitter
	_, err = unwired.GuardianHasChildAccess(ctx, 1, 7)
	require.ErrorIs(t, err, ErrEmitterNotConfigured, "an unwired emitter must error, never silently grant access")
}

// --- nil safety ---------------------------------------------------------

// TestEmitterShellIsNilSafe pins that every entry point a partially-wired
// service may hit tolerates a nil emitter and a nil port, and that the wired
// shell forwards to its Communication side.
func TestEmitterShellIsNilSafe(t *testing.T) {
	t.Parallel()

	var unwired *Emitter
	unwired.EmitChildEvent(1, 2, 3, ChildEvent{EventType: "sick_note"})
	unwired.BroadcastChildUpdateToGuardians(1, 2)
	unwired.EmitChildEventToGuardians(1, 2, []int64{3}, ChildEvent{})
	assert.True(t, unwired.MessagingEnabledForTenant(context.Background(), 1), "an unwired emitter never blocks a flow")
	require.NoError(t, unwired.EnqueueRequestDecision(context.Background(), 1, 2, 3, ChildEvent{}))

	bare := &Emitter{}
	bare.EmitChildEvent(1, 2, 3, ChildEvent{EventType: "sick_note"})
	bare.BroadcastChildUpdateToGuardians(1, 2)

	events := &fakeEvents{}
	wired := NewEmitter(events)
	wired.EmitChildEvent(1, 2, 3, ChildEvent{EventType: "sick_note"})
	wired.EmitChildEvent(0, 2, 3, ChildEvent{EventType: "sick_note"})
	wired.BroadcastChildUpdateToGuardians(1, 2)
	assert.Len(t, events.emitted, 1, "only a fully addressed event reaches the owner")
	assert.Equal(t, []int64{2}, events.woken)
}

// --- IsTerminalRequestEvent ---------------------------------------------

// TestIsTerminalRequestEvent pins the gate that lets a request-closing pill
// through EmitChildEvent while messaging is disabled: ONLY a request_status
// event that references the request row (ref_table + ref_id). Everything else —
// the request_created pill, self-service mirrors, and a status event with no ref
// — stays dropped, so a disabled school never accumulates threads or dangling
// pills.
func TestIsTerminalRequestEvent(t *testing.T) {
	t.Parallel()

	refID := int64(7)
	terminal := ChildEvent{
		EventType: EventRequestStatus,
		RefTable:  "schedule.care_schedule_change_requests",
		RefID:     &refID,
	}
	assert.True(t, IsTerminalRequestEvent(terminal), "a request_status event with a ref closes a request and must reconcile")

	assert.False(t, IsTerminalRequestEvent(ChildEvent{
		EventType: EventRequestCreated, RefTable: "schedule.care_schedule_change_requests", RefID: &refID,
	}), "a request_created pill carries no reconcile obligation")
	assert.False(t, IsTerminalRequestEvent(ChildEvent{
		EventType: "sick_note", RefTable: "x", RefID: &refID,
	}), "a self-service mirror is not a terminal request event")
	assert.False(t, IsTerminalRequestEvent(ChildEvent{
		EventType: EventRequestStatus, RefTable: "schedule.care_schedule_change_requests",
	}), "a status event without a ref_id cannot locate a created pill to close")
	assert.False(t, IsTerminalRequestEvent(ChildEvent{
		EventType: EventRequestStatus, RefID: &refID,
	}), "a status event without a ref_table cannot locate a created pill to close")
}
