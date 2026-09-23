package presence

import (
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
)

// BroadcastEvent is one realtime notification as the delivery producers shape
// it. Named here so a caller checking what these services announce does not
// have to reach past them to the delivery module or the transport.
type BroadcastEvent = realtimeevents.Event

// BroadcastEventType is the name an event carries.
type BroadcastEventType = realtimeevents.EventType

// The events these services publish, in the order a day produces them. They
// are aliases, so an event renamed downstream is a compile error here.
const (
	EventStudentCheckIn           = realtimeevents.EventStudentCheckIn
	EventStudentCheckOut          = realtimeevents.EventStudentCheckOut
	EventBulkStudentCheckIn       = realtimeevents.EventBulkStudentCheckIn
	EventBulkStudentCheckOut      = realtimeevents.EventBulkStudentCheckOut
	EventActivityStart            = realtimeevents.EventActivityStart
	EventActivityEnd              = realtimeevents.EventActivityEnd
	EventActiveSupervisionChanged = realtimeevents.EventActiveSupervisionChanged
	EventDashboardCountsChanged   = realtimeevents.EventDashboardCountsChanged
	EventStaffTimeTrackingChanged = realtimeevents.EventStaffTimeTrackingChanged
)
