package compose

import (
	"context"
	"sync"
)

// Package-local doubles of the lifecycle's consumer-owned ports
// (instance_lifecycle_ports.go, instance_broadcast.go). The shared realtime
// recorder (test.RecordingBroadcaster) serves realtime.Broadcaster, which
// the owner no longer names; these record the owner's own event shape.

// lifecycleBroadcastCall records one LifecycleBroadcaster invocation.
type lifecycleBroadcastCall struct {
	Method   string // "group" or "tenant"
	TenantID int64
	Topic    string // BroadcastToGroup only
	Event    LifecycleEvent
}

// lifecycleEventRecorder records every lifecycle event under a mutex.
type lifecycleEventRecorder struct {
	mu    sync.Mutex
	calls []lifecycleBroadcastCall
}

func (r *lifecycleEventRecorder) record(call lifecycleBroadcastCall) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
	return nil
}

func (r *lifecycleEventRecorder) BroadcastToGroup(tenantID int64, topic string, event LifecycleEvent) error {
	return r.record(lifecycleBroadcastCall{Method: "group", TenantID: tenantID, Topic: topic, Event: event})
}

func (r *lifecycleEventRecorder) BroadcastToTenant(tenantID int64, event LifecycleEvent) error {
	return r.record(lifecycleBroadcastCall{Method: "tenant", TenantID: tenantID, Event: event})
}

// Calls returns every recorded call, in order.
func (r *lifecycleEventRecorder) Calls() []lifecycleBroadcastCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]lifecycleBroadcastCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// GroupCallsForTopic returns every BroadcastToGroup call routed to the topic.
func (r *lifecycleEventRecorder) GroupCallsForTopic(topic string) []lifecycleBroadcastCall {
	out := make([]lifecycleBroadcastCall, 0)
	for _, call := range r.Calls() {
		if call.Method == "group" && call.Topic == topic {
			out = append(out, call)
		}
	}
	return out
}

// EventsOfType returns the events of every recorded call with the type.
func (r *lifecycleEventRecorder) EventsOfType(eventType string) []LifecycleEvent {
	out := make([]LifecycleEvent, 0)
	for _, call := range r.Calls() {
		if call.Event.Type == eventType {
			out = append(out, call.Event)
		}
	}
	return out
}

// recordingDeviationProtocol captures Änderungsprotokoll writes (#1886).
// Func-field convention: nil = zero-value default (record and succeed).
type recordingDeviationProtocol struct {
	events    []DeviationEventRecord
	createErr error
}

func (r *recordingDeviationProtocol) RecordDeviationEvent(_ context.Context, event DeviationEventRecord) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.events = append(r.events, event)
	return nil
}

// lifecycleSettingsStub answers the Settings Platform's clock policy.
type lifecycleSettingsStub struct {
	intVal  int
	intErr  error
	boolVal bool
	boolErr error
}

func (s lifecycleSettingsStub) StartLeadMinutes(context.Context) (int, error) {
	return s.intVal, s.intErr
}

func (s lifecycleSettingsStub) EnforcePlannedEnd(context.Context) (bool, error) {
	return s.boolVal, s.boolErr
}
