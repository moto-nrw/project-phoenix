package presence

import "github.com/moto-nrw/project-phoenix/realtime"

// BroadcastEvent is one realtime notification as the root shapes the
// announcements of these services. Named here so the suites checking what the
// services announce read the transport's events in one vocabulary.
type BroadcastEvent = realtime.Event

// BroadcastEventType is the name an event carries.
type BroadcastEventType = realtime.EventType

// The events these services publish, in the order a day produces them. They
// are aliases, so an event renamed downstream is a compile error here.
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
)
