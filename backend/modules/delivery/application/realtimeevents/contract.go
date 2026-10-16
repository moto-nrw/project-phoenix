package realtimeevents

import (
	"github.com/moto-nrw/project-phoenix/realtime"
)

// The event shapes and names the producers in this package emit. They are
// aliases rather than copies, so an event renamed on the transport is a
// compile error here, and a caller that only wants to know what this package
// broadcasts does not have to reach past it to the transport itself.
type (
	Event     = realtime.Event
	EventType = realtime.EventType
)

// Event names the producers in this package publish.
const (
	EventStudentCheckIn           = realtime.EventStudentCheckIn
	EventStudentCheckOut          = realtime.EventStudentCheckOut
	EventBulkStudentCheckIn       = realtime.EventBulkStudentCheckIn
	EventBulkStudentCheckOut      = realtime.EventBulkStudentCheckOut
	EventActivityStart            = realtime.EventActivityStart
	EventActivityEnd              = realtime.EventActivityEnd
	EventActiveSupervisionChanged = realtime.EventActiveSupervisionChanged
	EventDashboardCountsChanged   = realtime.EventDashboardCountsChanged
	EventStaffTimeTrackingChanged = realtime.EventStaffTimeTrackingChanged
	EventGroupAccessChanged       = realtime.EventGroupAccessChanged
)
