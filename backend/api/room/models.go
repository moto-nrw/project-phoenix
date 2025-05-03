package room

import (
	"errors"
	"net/http"
	"time"

	models2 "github.com/moto-nrw/project-phoenix/models"
)

// RoomRequest represents request payload for room data
type RoomRequest struct {
	*models2.Room
}

// Bind preprocesses a RoomRequest
func (rr *RoomRequest) Bind(r *http.Request) error {
	// Simple validation - more detailed validation in ValidateRoom
	if rr.Room == nil {
		return errors.New("missing room data")
	}
	return nil
}

// RegisterTabletRequest represents request payload for tablet registration
type RegisterTabletRequest struct {
	DeviceID   string `json:"device_id"`
	GroupID    *int64 `json:"group_id,omitempty"`
	AgID       *int64 `json:"ag_id,omitempty"`
	ActivityID *int64 `json:"activity_id,omitempty"`
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
	if r.DeviceID == "" {
		return errors.New("device_id is required")
	}

	// At least one of GroupID or AgID should be provided
	if r.GroupID == nil && r.AgID == nil {
		return errors.New("at least one of group_id or ag_id is required")
	}

	return nil
}

// UnregisterTabletRequest represents request payload for tablet unregistration
type UnregisterTabletRequest struct {
	DeviceID string `json:"device_id"`
}

// Bind preprocesses an UnregisterTabletRequest
func (r *UnregisterTabletRequest) Bind(req *http.Request) error {
	if r.DeviceID == "" {
		return errors.New("device_id is required")
	}
	return nil
}

// MergeRoomsRequest represents request payload for merging rooms
type MergeRoomsRequest struct {
	SourceRoomID int64      `json:"source_room_id"`
	TargetRoomID int64      `json:"target_room_id"`
	Name         string     `json:"name,omitempty"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	AccessPolicy string     `json:"access_policy,omitempty"`
}

// Bind preprocesses a MergeRoomsRequest
func (m *MergeRoomsRequest) Bind(r *http.Request) error {
	// Validate required fields
	if m.SourceRoomID == 0 {
		return errors.New("source_room_id is required")
	}
	if m.TargetRoomID == 0 {
		return errors.New("target_room_id is required")
	}

	// Validate that source and target are not the same
	if m.SourceRoomID == m.TargetRoomID {
		return errors.New("source and target rooms must be different")
	}

	// Validate access policy if provided
	if m.AccessPolicy != "" &&
		m.AccessPolicy != "all" &&
		m.AccessPolicy != "first" &&
		m.AccessPolicy != "specific" &&
		m.AccessPolicy != "manual" {
		return errors.New("access_policy must be one of: all, first, specific, manual")
	}

	return nil
}

// RoomOccupancyResponse represents a formatted response for room occupancy
type RoomOccupancyResponse struct {
	RoomID       int64             `json:"room_id"`
	RoomName     string            `json:"room_name"`
	Building     string            `json:"building,omitempty"`
	Floor        int               `json:"floor"`
	StartTime    string            `json:"start_time"`
	EndTime      *string           `json:"end_time,omitempty"`
	Supervisors  []SupervisorInfo  `json:"supervisors,omitempty"`
	Activity     *ActivityInfo     `json:"activity,omitempty"`
	Group        *GroupInfo        `json:"group,omitempty"`
	Participants []ParticipantInfo `json:"participants,omitempty"`
}

// SupervisorInfo represents supervisor information in the occupancy response
type SupervisorInfo struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// ActivityInfo represents activity information in the occupancy response
type ActivityInfo struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
}

// GroupInfo represents group information in the occupancy response
type GroupInfo struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// ParticipantInfo represents participant information in the occupancy response
type ParticipantInfo struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// RoomHistoryResponse represents a formatted response for room history
type RoomHistoryResponse struct {
	ID           int64             `json:"id"`
	RoomID       int64             `json:"room_id"`
	RoomName     string            `json:"room_name"`
	Day          string            `json:"day"`
	StartTime    string            `json:"start_time"`
	EndTime      string            `json:"end_time,omitempty"`
	Activity     string            `json:"activity"`
	Category     string            `json:"category,omitempty"`
	Supervisor   SupervisorInfo    `json:"supervisor"`
	Participants []ParticipantInfo `json:"participants,omitempty"`
}

// CombinedGroupResponse represents a formatted response for combined group
type CombinedGroupResponse struct {
	ID           int64            `json:"id"`
	Name         string           `json:"name"`
	IsActive     bool             `json:"is_active"`
	ValidUntil   *string          `json:"valid_until,omitempty"`
	AccessPolicy string           `json:"access_policy"`
	Groups       []GroupInfo      `json:"groups"`
	Supervisors  []SupervisorInfo `json:"supervisors"`
}
