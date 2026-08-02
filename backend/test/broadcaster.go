package test

import (
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BroadcastCall records one realtime.Broadcaster invocation.
type BroadcastCall struct {
	Method     string // "group", "tenant", "admin", "all", "parent", "guardian" or "staff"
	TenantID   int64
	Topic      string  // active-group topic (BroadcastToGroup only)
	GuardianID int64   // guardian account ID (BroadcastParentMessage / BroadcastToGuardian)
	AccountIDs []int64 // staff account IDs (BroadcastToStaffAccounts only)
	Event      realtime.Event
}

// RecordingBroadcaster is a shared test double for realtime.Broadcaster.
// It records every call under a mutex; set Err to inject a broadcast failure.
type RecordingBroadcaster struct {
	mu    sync.Mutex
	calls []BroadcastCall

	// Err, when non-nil, is returned by every Broadcast* method (the calls
	// are still recorded, mirroring the fire-and-forget production contract).
	Err error
}

// NewRecordingBroadcaster creates an empty recording broadcaster.
func NewRecordingBroadcaster() *RecordingBroadcaster {
	return &RecordingBroadcaster{}
}

func (b *RecordingBroadcaster) record(c BroadcastCall) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, c)
	return b.Err
}

// BroadcastToGroup implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastToGroup(tenantID int64, topic string, event realtime.Event) error {
	return b.record(BroadcastCall{Method: "group", TenantID: tenantID, Topic: topic, Event: event})
}

// BroadcastToTenant implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastToTenant(tenantID int64, event realtime.Event) error {
	return b.record(BroadcastCall{Method: "tenant", TenantID: tenantID, Event: event})
}

// BroadcastToStaffAccounts implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastToStaffAccounts(tenantID int64, accountIDs []int64, event realtime.Event) error {
	return b.record(BroadcastCall{Method: "staff", TenantID: tenantID, AccountIDs: accountIDs, Event: event})
}

// BroadcastToTenantAdmins implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastToTenantAdmins(tenantID int64, event realtime.Event) error {
	return b.record(BroadcastCall{Method: "admin", TenantID: tenantID, Event: event})
}

// BroadcastToAll implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastToAll(event realtime.Event) error {
	return b.record(BroadcastCall{Method: "all", Event: event})
}

// BroadcastParentMessage implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastParentMessage(tenantID, guardianAccountID int64, event realtime.Event) error {
	return b.record(BroadcastCall{Method: "parent", TenantID: tenantID, GuardianID: guardianAccountID, Event: event})
}

// BroadcastToGuardian implements realtime.Broadcaster.
func (b *RecordingBroadcaster) BroadcastToGuardian(tenantID, guardianAccountID int64, event realtime.Event) error {
	return b.record(BroadcastCall{Method: "guardian", TenantID: tenantID, GuardianID: guardianAccountID, Event: event})
}

// Calls returns a copy of every recorded call, in order.
func (b *RecordingBroadcaster) Calls() []BroadcastCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]BroadcastCall, len(b.calls))
	copy(out, b.calls)
	return out
}

// CallsByMethod returns every recorded call made through the given method
// ("group", "tenant", "admin", "all", "parent" or "guardian").
func (b *RecordingBroadcaster) CallsByMethod(method string) []BroadcastCall {
	out := make([]BroadcastCall, 0)
	for _, c := range b.Calls() {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// GroupCallsForTopic returns every BroadcastToGroup call routed to the topic.
func (b *RecordingBroadcaster) GroupCallsForTopic(topic string) []BroadcastCall {
	out := make([]BroadcastCall, 0)
	for _, c := range b.CallsByMethod("group") {
		if c.Topic == topic {
			out = append(out, c)
		}
	}
	return out
}

// Events returns the events of every recorded call, in order.
func (b *RecordingBroadcaster) Events() []realtime.Event {
	calls := b.Calls()
	out := make([]realtime.Event, len(calls))
	for i, c := range calls {
		out[i] = c.Event
	}
	return out
}

// EventsOfType returns every recorded event with the given type.
func (b *RecordingBroadcaster) EventsOfType(t realtime.EventType) []realtime.Event {
	out := make([]realtime.Event, 0)
	for _, e := range b.Events() {
		if e.Type == t {
			out = append(out, e)
		}
	}
	return out
}

// HasEventType reports whether any recorded event has the given type.
func (b *RecordingBroadcaster) HasEventType(t realtime.EventType) bool {
	return len(b.EventsOfType(t)) > 0
}

// AssertSingleTenantEvent asserts that exactly one event of the given type was
// broadcast tenant-wide to tenantID, and returns it so the caller can assert
// its payload. It also pins the shared privacy contract of the tenant-wide
// invalidation events: they reach every staff client of a school, so they must
// carry no student or staff identity.
//
// Lives here rather than in each API test package because the same five lines
// were being re-hand-rolled per package, and the copies drifted — each one
// checked a different identity field, so each covered half the contract.
func AssertSingleTenantEvent(tb testing.TB, b *RecordingBroadcaster, eventType realtime.EventType, tenantID int64) realtime.Event {
	tb.Helper()

	events := b.EventsOfType(eventType)
	require.Len(tb, events, 1, "expected exactly one %s event", eventType)

	event := events[0]
	assert.Empty(tb, event.ActiveGroupID, "%s is tenant-wide, not group-scoped", eventType)
	assert.Nil(tb, event.Data.StudentID, "%s must not carry student identity", eventType)
	assert.Nil(tb, event.Data.StudentIDs, "%s must not carry student identity", eventType)
	assert.Nil(tb, event.Data.SupervisorIDs, "%s must not carry staff identity", eventType)

	calls := b.CallsByMethod("tenant")
	require.NotEmpty(tb, calls, "expected a tenant-scoped broadcast")
	assert.Equal(tb, tenantID, calls[0].TenantID)

	return event
}

// AssertNoTenantWideStudentIdentity asserts that not one of the recorded
// BroadcastToTenant calls carries a child identifier — the invariant behind
// #2085. A tenant-scoped broadcast lands on EVERY staff client of the school,
// including colleagues whose gdpr.student_data_scope keeps that child's data
// out of their API responses; a student id in the payload hands them the
// movement fact anyway. Child-identifying payloads belong on the group-scoped
// topics (BroadcastToGroup) or on a guardian's own channel.
//
// Deliberately broader than AssertSingleTenantEvent, which pins one specific
// event: this one sweeps everything a flow emitted, so a NEW tenant-wide event
// added to that flow later is covered without anyone remembering to extend the
// test.
func AssertNoTenantWideStudentIdentity(tb testing.TB, b *RecordingBroadcaster) {
	tb.Helper()

	for _, call := range b.CallsByMethod("tenant") {
		assert.Nil(tb, call.Event.Data.StudentID,
			"tenant-wide %s must not carry a student id", call.Event.Type)
		assert.Nil(tb, call.Event.Data.StudentIDs,
			"tenant-wide %s must not carry student ids", call.Event.Type)
		assert.Nil(tb, call.Event.Data.StudentName,
			"tenant-wide %s must not carry a student name", call.Event.Type)
	}
}

// Reset drops all recorded calls.
func (b *RecordingBroadcaster) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = nil
}
