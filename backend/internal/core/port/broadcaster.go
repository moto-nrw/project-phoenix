// Package port defines interfaces (contracts) for adapters.
// These interfaces are defined by what the domain/service layer needs,
// following the Dependency Inversion Principle.
package port

import "time"

// EventType represents the type of real-time event.
type EventType string

// Event type constants for real-time notifications.
const (
	// Student movement events
	EventStudentCheckIn  EventType = "student_checkin"
	EventStudentCheckOut EventType = "student_checkout"

	// Activity session lifecycle events
	EventActivityStart  EventType = "activity_start"
	EventActivityEnd    EventType = "activity_end"
	EventActivityUpdate EventType = "activity_update"
)

// Event represents a real-time event to be broadcast to clients.
// This is a domain concept independent of the transport mechanism (SSE, WebSocket, etc).
type Event struct {
	Type          EventType `json:"type"`
	ActiveGroupID string    `json:"active_group_id"`
	Data          EventData `json:"data"`
	Timestamp     time.Time `json:"timestamp"`
}

// EventData contains the payload for a real-time event.
// Only includes display-level data for GDPR compliance (no sensitive info).
type EventData struct {
	// Student-related fields (for check-in/check-out events)
	StudentID   *string `json:"student_id,omitempty"`
	StudentName *string `json:"student_name,omitempty"`
	SchoolClass *string `json:"school_class,omitempty"`
	GroupName   *string `json:"group_name,omitempty"` // Student's OGS group, not active group

	// Activity session fields (for activity_start/end/update events)
	ActivityName  *string   `json:"activity_name,omitempty"`
	RoomID        *string   `json:"room_id,omitempty"`
	RoomName      *string   `json:"room_name,omitempty"`
	SupervisorIDs *[]string `json:"supervisor_ids,omitempty"`

	// Source tracking
	Source *string `json:"source,omitempty"` // "rfid", "manual", "automated"
}

// NewEvent creates a new event with current timestamp.
func NewEvent(eventType EventType, activeGroupID string, data EventData) Event {
	return Event{
		Type:          eventType,
		ActiveGroupID: activeGroupID,
		Data:          data,
		Timestamp:     time.Now(),
	}
}

// Broadcaster defines the interface for broadcasting events to real-time clients.
// Services use this interface to emit events without depending on implementation details.
// This follows the Hexagonal Architecture pattern where the domain defines what it needs.
type Broadcaster interface {
	// BroadcastToGroup sends an event to all clients subscribed to the given active group ID.
	// This is a fire-and-forget operation - errors are logged but don't affect service execution.
	BroadcastToGroup(activeGroupID string, event Event) error
}
