package room

import (
	"net/http"
)

// Use the models.RoomOccupancy and models.RoomOccupancySupervisor directly

// RegisterTabletRequest represents request to register a tablet to a room
type RegisterTabletRequest struct {
	DeviceID    string  `json:"device_id"`
	Supervisors []int64 `json:"supervisors"`
	GroupID     *int64  `json:"group,omitempty"`
	AgID        *int64  `json:"ag_id,omitempty"`
	NewAg       *NewAg  `json:"ag,omitempty"`
}

// NewAg contains information for creating a new activity group during room registration
type NewAg struct {
	Name           string `json:"name"`
	MaxParticipant int    `json:"max_participant"`
	AgCategoryID   int64  `json:"ag_category"`
	IsOpenAgs      bool   `json:"is_open_ags"`
}

// Bind preprocesses a RegisterTabletRequest
func (r *RegisterTabletRequest) Bind(req *http.Request) error {
	// Validation logic (at least one of GroupID, AgID, or NewAg should be provided)
	return nil
}

// UnregisterTabletRequest represents request to unregister a tablet from a room
type UnregisterTabletRequest struct {
	DeviceID string `json:"device_id"`
}

// Bind preprocesses a UnregisterTabletRequest
func (u *UnregisterTabletRequest) Bind(req *http.Request) error {
	// Validation logic
	return nil
}

// RoomOccupancyDetail represents the detailed view of room occupancy
type RoomOccupancyDetail struct {
	Room       RoomInfo         `json:"room"`
	Ag         *AgInfo          `json:"ag,omitempty"`
	Supervisor []SupervisorInfo `json:"supervisor"`
	Timespan   TimespanInfo     `json:"timespan"`
}

// Supporting structs for RoomOccupancyDetail
type RoomInfo struct {
	RoomName string `json:"room_name"`
	Floor    int    `json:"floor"`
	Capacity int    `json:"capacity"`
}

type AgInfo struct {
	Name           string `json:"name"`
	Category       string `json:"category,omitempty"`
	MaxParticipant int    `json:"max_participant"`
}

type SupervisorInfo struct {
	ID         int64  `json:"id"`
	FirstName  string `json:"first_name"`
	SecondName string `json:"second_name"`
}

type TimespanInfo struct {
	StartTime string `json:"starttime"`
	EndTime   string `json:"endtime,omitempty"`
}
