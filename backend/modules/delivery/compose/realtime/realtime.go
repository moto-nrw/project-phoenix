// Package realtime names the Delivery live-event vocabulary for the legacy composition (#2747): the broadcaster, the hub and the events it publishes.
package realtime

import upstream "github.com/moto-nrw/project-phoenix/realtime"

type (
	Broadcaster = upstream.Broadcaster
	Event       = upstream.Event
	EventData   = upstream.EventData
	EventType   = upstream.EventType
	Hub         = upstream.Hub
)

const (
	EventActiveSupervisionChanged = upstream.EventActiveSupervisionChanged
	EventActivityUpdate           = upstream.EventActivityUpdate
	EventArrivalScheduleChanged   = upstream.EventArrivalScheduleChanged
	EventChangeRequestsChanged    = upstream.EventChangeRequestsChanged
	EventInstanceStarted          = upstream.EventInstanceStarted
	EventPickupScheduleChanged    = upstream.EventPickupScheduleChanged
	EventStaffTimeTrackingChanged = upstream.EventStaffTimeTrackingChanged
	EventStaffingDeviationChanged = upstream.EventStaffingDeviationChanged
	EventStudentCompanionsChanged = upstream.EventStudentCompanionsChanged
	EventStudentUpdated           = upstream.EventStudentUpdated
	EventTenantSettingsChanged    = upstream.EventTenantSettingsChanged
)

var (
	NewEvent = upstream.NewEvent
)
