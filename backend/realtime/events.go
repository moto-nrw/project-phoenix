// Package realtime provides Server-Sent Events (SSE) infrastructure for real-time notifications.
// This package is dependency-neutral to avoid circular imports between api and services layers.
package realtime

import (
	"time"

	locationModels "github.com/moto-nrw/project-phoenix/models/location"
)

// EventType represents the type of SSE event
type EventType string

// Event type constants
const (
	// Student movement events
	EventStudentCheckIn  EventType = "student_checkin"
	EventStudentCheckOut EventType = "student_checkout"

	// Activity session lifecycle events
	EventActivityStart  EventType = "activity_start"
	EventActivityEnd    EventType = "activity_end"
	EventActivityUpdate EventType = "activity_update"
)

// Event represents a Server-Sent Event that will be broadcast to clients
type Event struct {
	Type          EventType `json:"type"`
	ActiveGroupID string    `json:"active_group_id"`
	Data          EventData `json:"data"`
	Timestamp     time.Time `json:"timestamp"`
}

// EventData contains the payload for an SSE event
// Only includes display-level data for GDPR compliance (no sensitive info)
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

	// Structured student location
	LocationStatus *locationModels.Status `json:"location_status,omitempty"`
}

// NewEvent creates a new event with current timestamp
func NewEvent(eventType EventType, activeGroupID string, data EventData) Event {
	return Event{
		Type:          eventType,
		ActiveGroupID: activeGroupID,
		Data:          data,
		Timestamp:     time.Now(),
	}
}
