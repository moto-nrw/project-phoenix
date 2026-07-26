package test

import (
	"sync"

	"github.com/moto-nrw/project-phoenix/realtime"
)

// BroadcastCall records one realtime.Broadcaster invocation.
type BroadcastCall struct {
	Method     string // "group", "tenant", "admin", "all", "parent" or "guardian"
	TenantID   int64
	Topic      string // active-group topic (BroadcastToGroup only)
	GuardianID int64  // guardian account ID (BroadcastParentMessage / BroadcastToGuardian)
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

// Reset drops all recorded calls.
func (b *RecordingBroadcaster) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = nil
}
