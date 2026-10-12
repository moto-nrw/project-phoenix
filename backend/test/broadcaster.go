package test

import "github.com/moto-nrw/project-phoenix/realtime/realtimetest"

// The broadcaster test double lives with the Delivery Platform
// (realtimetest); these aliases keep the suites that take it from the
// fixture catalog unchanged.
type BroadcastCall = realtimetest.BroadcastCall
type RecordingBroadcaster = realtimetest.RecordingBroadcaster

var (
	NewRecordingBroadcaster           = realtimetest.NewRecordingBroadcaster
	AssertSingleTenantEvent           = realtimetest.AssertSingleTenantEvent
	AssertNoTenantWideStudentIdentity = realtimetest.AssertNoTenantWideStudentIdentity
)
