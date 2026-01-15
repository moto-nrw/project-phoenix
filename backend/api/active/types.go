package active

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// ===== Response Types =====

// ActiveGroupResponse represents an active group API response
type ActiveGroupResponse struct {
	ID              int64                   `json:"id"`
	GroupID         int64                   `json:"group_id"`
	RoomID          int64                   `json:"room_id"`
	StartTime       time.Time               `json:"start_time"`
	EndTime         *time.Time              `json:"end_time,omitempty"`
	IsActive        bool                    `json:"is_active"`
	Notes           string                  `json:"notes,omitempty"`
	VisitCount      int                     `json:"visit_count,omitempty"`
	SupervisorCount int                     `json:"supervisor_count,omitempty"`
	Supervisors     []GroupSupervisorSimple `json:"supervisors,omitempty"`
	Room            *RoomSimple             `json:"room,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// GroupSupervisorSimple represents simplified supervisor info for active group response
type GroupSupervisorSimple struct {
	StaffID int64  `json:"staff_id"`
	Role    string `json:"role,omitempty"`
}

// RoomSimple represents simplified room info for active group response
type RoomSimple struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// SupervisorResponse represents a group supervisor API response
type SupervisorResponse struct {
	ID              int64      `json:"id"`
	StaffID         int64      `json:"staff_id"`
	ActiveGroupID   int64      `json:"active_group_id"`
	StartTime       time.Time  `json:"start_time"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	IsActive        bool       `json:"is_active"`
	Notes           string     `json:"notes,omitempty"`
	StaffName       string     `json:"staff_name,omitempty"`
	ActiveGroupName string     `json:"active_group_name,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CombinedGroupResponse represents a combined group API response
type CombinedGroupResponse struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	RoomID      int64      `json:"room_id"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	IsActive    bool       `json:"is_active"`
	Notes       string     `json:"notes,omitempty"`
	GroupCount  int        `json:"group_count,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// GroupMappingResponse represents a group mapping API response
type GroupMappingResponse struct {
	ID              int64  `json:"id"`
	ActiveGroupID   int64  `json:"active_group_id"`
	CombinedGroupID int64  `json:"combined_group_id"`
	GroupName       string `json:"group_name,omitempty"`
	CombinedName    string `json:"combined_name,omitempty"`
}

// ===== Request Types =====

// ActiveGroupRequest represents an active group creation/update request
type ActiveGroupRequest struct {
	GroupID   int64      `json:"group_id"`
	RoomID    int64      `json:"room_id"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Notes     string     `json:"notes,omitempty"`
}

// SupervisorRequest represents a group supervisor creation/update request
type SupervisorRequest struct {
	StaffID       int64      `json:"staff_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	StartTime     time.Time  `json:"start_time"`
	EndTime       *time.Time `json:"end_time,omitempty"`
	Notes         string     `json:"notes,omitempty"`
}

// CombinedGroupRequest represents a combined group creation/update request
type CombinedGroupRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	RoomID      int64      `json:"room_id"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Notes       string     `json:"notes,omitempty"`
	GroupIDs    []int64    `json:"group_ids,omitempty"`
}

// GroupMappingRequest represents a group mapping creation request
type GroupMappingRequest struct {
	ActiveGroupID   int64 `json:"active_group_id"`
	CombinedGroupID int64 `json:"combined_group_id"`
}

// ===== Request Binding Functions =====

// Bind validates the active group request
func (req *ActiveGroupRequest) Bind(_ *http.Request) error {
	if req.GroupID <= 0 {
		return errors.New("group ID is required")
	}
	if req.RoomID <= 0 {
		return errors.New("room ID is required")
	}
	if req.StartTime.IsZero() {
		return errors.New(errMsgStartTimeRequired)
	}
	return nil
}

// Bind validates the supervisor request
func (req *SupervisorRequest) Bind(_ *http.Request) error {
	if req.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if req.ActiveGroupID <= 0 {
		return errors.New(errMsgActiveGroupIDRequired)
	}
	if req.StartTime.IsZero() {
		return errors.New(errMsgStartTimeRequired)
	}
	return nil
}

// Bind validates the combined group request
func (req *CombinedGroupRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.RoomID <= 0 {
		return errors.New("room ID is required")
	}
	if req.StartTime.IsZero() {
		return errors.New(errMsgStartTimeRequired)
	}
	return nil
}

// Bind validates the group mapping request
func (req *GroupMappingRequest) Bind(_ *http.Request) error {
	if req.ActiveGroupID <= 0 {
		return errors.New(errMsgActiveGroupIDRequired)
	}
	if req.CombinedGroupID <= 0 {
		return errors.New("combined group ID is required")
	}
	return nil
}

// ===== Response Builder Functions =====

// newActiveGroupResponse converts an active group model to a response object
func newActiveGroupResponse(group *active.Group) ActiveGroupResponse {
	response := ActiveGroupResponse{
		ID:        group.ID,
		GroupID:   group.GroupID,
		RoomID:    group.RoomID,
		StartTime: group.StartTime,
		EndTime:   group.EndTime,
		IsActive:  group.IsActive(),
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}

	// Add counts if available
	if group.Visits != nil {
		response.VisitCount = len(group.Visits)
	}
	if group.Supervisors != nil {
		// Only expose currently active supervisors
		activeSupervisors := make([]*active.GroupSupervisor, 0, len(group.Supervisors))
		for _, supervisor := range group.Supervisors {
			if supervisor.IsActive() {
				activeSupervisors = append(activeSupervisors, supervisor)
			}
		}

		response.SupervisorCount = len(activeSupervisors)
		// Add supervisor details
		response.Supervisors = make([]GroupSupervisorSimple, 0, len(activeSupervisors))
		for _, supervisor := range activeSupervisors {
			response.Supervisors = append(response.Supervisors, GroupSupervisorSimple{
				StaffID: supervisor.StaffID,
				Role:    supervisor.Role,
			})
		}
	}

	// Add room info if available
	if group.Room != nil {
		response.Room = &RoomSimple{
			ID:   group.Room.ID,
			Name: group.Room.Name,
		}
	}

	return response
}

// newSupervisorResponse converts a group supervisor model to a response object
func newSupervisorResponse(supervisor *active.GroupSupervisor) SupervisorResponse {
	response := SupervisorResponse{
		ID:            supervisor.ID,
		StaffID:       supervisor.StaffID,
		ActiveGroupID: supervisor.GroupID,
		StartTime:     supervisor.StartDate,
		EndTime:       supervisor.EndDate,
		IsActive:      supervisor.IsActive(),
		CreatedAt:     supervisor.CreatedAt,
		UpdatedAt:     supervisor.UpdatedAt,
	}

	// Add related information if available
	if supervisor.Staff != nil && supervisor.Staff.Person != nil {
		response.StaffName = supervisor.Staff.Person.GetFullName()
	}
	if supervisor.ActiveGroup != nil {
		response.ActiveGroupName = displayGroupPrefix + strconv.FormatInt(supervisor.ActiveGroup.GroupID, 10)
	}

	return response
}

// newCombinedGroupResponse converts a combined group model to a response object
func newCombinedGroupResponse(group *active.CombinedGroup) CombinedGroupResponse {
	response := CombinedGroupResponse{
		ID:          group.ID,
		Name:        "Combined Group #" + strconv.FormatInt(group.ID, 10), // Using ID as name since the model doesn't have name
		Description: "",                                                   // Using empty description since the model doesn't have description
		RoomID:      0,                                                    // Using default value since the model doesn't have roomID
		StartTime:   group.StartTime,
		EndTime:     group.EndTime,
		IsActive:    group.IsActive(),
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}

	// Add group count if available
	if group.ActiveGroups != nil {
		response.GroupCount = len(group.ActiveGroups)
	}

	return response
}

// newGroupMappingResponse converts a group mapping model to a response object
func newGroupMappingResponse(mapping *active.GroupMapping) GroupMappingResponse {
	response := GroupMappingResponse{
		ID:              mapping.ID,
		ActiveGroupID:   mapping.ActiveGroupID,
		CombinedGroupID: mapping.ActiveCombinedGroupID,
	}

	// Add related information if available
	if mapping.ActiveGroup != nil {
		response.GroupName = displayGroupPrefix + strconv.FormatInt(mapping.ActiveGroup.GroupID, 10)
	}
	if mapping.CombinedGroup != nil {
		response.CombinedName = "Combined Group #" + strconv.FormatInt(mapping.CombinedGroup.ID, 10)
	}

	return response
}
