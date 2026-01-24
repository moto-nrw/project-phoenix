package active

import (
	"errors"
	"net/http"
	"time"
)

// Route path constants
const (
	routeGroupByGroupID = "/group/{groupId}"
	routeEndByID        = "/{id}/end"
)

// Validation error messages
const (
	errMsgStartTimeRequired      = "start time is required"
	errMsgActiveGroupIDRequired  = "active group ID is required"
	errMsgInvalidActiveGroupID   = "invalid active group ID"
	errMsgInvalidGroupID         = "invalid group ID"
	errMsgInvalidVisitID         = "invalid visit ID"
	errMsgInvalidStudentID       = "invalid student ID"
	errMsgInvalidSupervisorID    = "invalid supervisor ID"
	errMsgInvalidCombinedGroupID = "invalid combined group ID"
)

// Display text constants
const (
	displayGroupPrefix = "Group #"
)

// Response messages
const (
	msgGroupAddedToCombination = "Group added to combination successfully"
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

// VisitResponse represents a visit API response
type VisitResponse struct {
	ID              int64      `json:"id"`
	StudentID       int64      `json:"student_id"`
	ActiveGroupID   int64      `json:"active_group_id"`
	CheckInTime     time.Time  `json:"check_in_time"`
	CheckOutTime    *time.Time `json:"check_out_time,omitempty"`
	IsActive        bool       `json:"is_active"`
	Notes           string     `json:"notes,omitempty"`
	StudentName     string     `json:"student_name,omitempty"`
	ActiveGroupName string     `json:"active_group_name,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// VisitWithDisplayDataResponse represents a visit with student display data (optimized for bulk fetch)
type VisitWithDisplayDataResponse struct {
	ID            int64      `json:"id"`
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	CheckInTime   time.Time  `json:"check_in_time"`
	CheckOutTime  *time.Time `json:"check_out_time,omitempty"`
	IsActive      bool       `json:"is_active"`
	StudentName   string     `json:"student_name"`
	SchoolClass   string     `json:"school_class"`
	GroupName     string     `json:"group_name,omitempty"` // Student's OGS group
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
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

// AnalyticsResponse represents analytics API response
type AnalyticsResponse struct {
	ActiveGroupsCount int     `json:"active_groups_count,omitempty"`
	TotalVisitsCount  int     `json:"total_visits_count,omitempty"`
	ActiveVisitsCount int     `json:"active_visits_count,omitempty"`
	RoomUtilization   float64 `json:"room_utilization,omitempty"`
	AttendanceRate    float64 `json:"attendance_rate,omitempty"`
}

// DashboardAnalyticsResponse represents dashboard analytics API response
type DashboardAnalyticsResponse struct {
	// Student Overview
	StudentsPresent      int `json:"students_present"`
	StudentsInTransit    int `json:"students_in_transit"` // Students present but not in any active visit
	StudentsOnPlayground int `json:"students_on_playground"`
	StudentsInRooms      int `json:"students_in_rooms"` // Students in indoor rooms (excluding playground)

	// Activities & Rooms
	ActiveActivities    int     `json:"active_activities"`
	FreeRooms           int     `json:"free_rooms"`
	TotalRooms          int     `json:"total_rooms"`
	CapacityUtilization float64 `json:"capacity_utilization"`
	ActivityCategories  int     `json:"activity_categories"`

	// OGS Groups
	ActiveOGSGroups      int `json:"active_ogs_groups"`
	StudentsInGroupRooms int `json:"students_in_group_rooms"`
	SupervisorsToday     int `json:"supervisors_today"`
	StudentsInHomeRoom   int `json:"students_in_home_room"`

	// Recent Activity (Privacy-compliant)
	RecentActivity []RecentActivityItem `json:"recent_activity"`

	// Current Activities (No personal data)
	CurrentActivities []CurrentActivityItem `json:"current_activities"`

	// Active Groups Summary
	ActiveGroupsSummary []ActiveGroupSummary `json:"active_groups_summary"`

	// Timestamp
	LastUpdated time.Time `json:"last_updated"`
}

// RecentActivityItem represents a recent activity without personal data
type RecentActivityItem struct {
	Type      string    `json:"type"`
	GroupName string    `json:"group_name"`
	RoomName  string    `json:"room_name"`
	Count     int       `json:"count"`
	Timestamp time.Time `json:"timestamp"`
}

// CurrentActivityItem represents current activity status
type CurrentActivityItem struct {
	Name         string `json:"name"`
	Category     string `json:"category"`
	Participants int    `json:"participants"`
	MaxCapacity  int    `json:"max_capacity"`
	Status       string `json:"status"`
}

// ActiveGroupSummary represents active group summary
type ActiveGroupSummary struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	StudentCount int    `json:"student_count"`
	Location     string `json:"location"`
	Status       string `json:"status"`
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

// VisitRequest represents a visit creation/update request
type VisitRequest struct {
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	CheckInTime   time.Time  `json:"check_in_time"`
	CheckOutTime  *time.Time `json:"check_out_time,omitempty"`
	Notes         string     `json:"notes,omitempty"`
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

// Bind validates the visit request
func (req *VisitRequest) Bind(_ *http.Request) error {
	if req.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if req.ActiveGroupID <= 0 {
		return errors.New(errMsgActiveGroupIDRequired)
	}
	if req.CheckInTime.IsZero() {
		return errors.New("check-in time is required")
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
