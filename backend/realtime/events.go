// Package realtime provides Server-Sent Events (SSE) infrastructure for real-time notifications.
// This package is an adapter that implements the port.Broadcaster interface.
//
// Type aliases are provided for backward compatibility - all canonical types
// are defined in internal/core/port/broadcaster.go following Hexagonal Architecture.
package realtime

import (
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// Type aliases for backward compatibility.
// These allow existing code to continue using realtime.Event, realtime.EventType, etc.
// New code should prefer importing from internal/core/port directly.
type (
	EventType = port.EventType
	Event     = port.Event
	EventData = port.EventData
)

// Event type constants - aliases to port constants for backward compatibility.
const (
	EventStudentCheckIn  = port.EventStudentCheckIn
	EventStudentCheckOut = port.EventStudentCheckOut
	EventActivityStart   = port.EventActivityStart
	EventActivityEnd     = port.EventActivityEnd
	EventActivityUpdate  = port.EventActivityUpdate
)

// NewEvent creates a new event with current timestamp.
// Delegates to port.NewEvent for implementation.
func NewEvent(eventType EventType, activeGroupID string, data EventData) Event {
	return port.NewEvent(eventType, activeGroupID, data)
}
