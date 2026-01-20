package active

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the active API resource
type Resource struct {
	ActiveService activeSvc.Service
	PersonService userSvc.PersonService
	AuthService   authSvc.AuthService
	db            *bun.DB
}

// NewResource creates a new active resource
func NewResource(activeService activeSvc.Service, personService userSvc.PersonService, authService authSvc.AuthService, db *bun.DB) *Resource {
	return &Resource{
		ActiveService: activeService,
		PersonService: personService,
		AuthService:   authService,
		db:            db,
	}
}

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

// Router returns a configured router for active endpoints
// Note: Authentication is handled by tenant middleware in base.go when TENANT_AUTH_ENABLED=true
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Active Groups
	r.Route("/groups", func(r chi.Router) {
		// Read operations
		r.With(tenant.RequiresPermission("group:read")).Get("/", rs.listActiveGroups)
		r.With(tenant.RequiresPermission("group:read")).Get("/unclaimed", rs.listUnclaimedGroups)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}", rs.getActiveGroup)
		r.With(tenant.RequiresPermission("group:read")).Get("/room/{roomId}", rs.getActiveGroupsByRoom)
		r.With(tenant.RequiresPermission("group:read")).Get(routeGroupByGroupID, rs.getActiveGroupsByGroup)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/visits", rs.getActiveGroupVisits)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/visits/display", rs.getActiveGroupVisitsWithDisplay)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/supervisors", rs.getActiveGroupSupervisors)

		// Write operations
		r.With(tenant.RequiresPermission("group:create")).Post("/", rs.createActiveGroup)
		r.With(tenant.RequiresPermission("group:update")).Put("/{id}", rs.updateActiveGroup)
		r.With(tenant.RequiresPermission("group:delete")).Delete("/{id}", rs.deleteActiveGroup)
		r.With(tenant.RequiresPermission("group:update")).Post(routeEndByID, rs.endActiveGroup)
		r.With(tenant.RequiresPermission("group:update")).Post("/{id}/claim", rs.claimGroup)
	})

	// Visits
	r.Route("/visits", func(r chi.Router) {
		// Read operations
		r.With(tenant.RequiresPermission("attendance:read")).Get("/", rs.listVisits)
		r.With(tenant.RequiresPermission("attendance:read")).Get("/{id}", rs.getVisit)
		r.With(tenant.RequiresPermission("attendance:read")).Get("/student/{studentId}", rs.getStudentVisits)
		r.With(tenant.RequiresPermission("attendance:read")).Get("/student/{studentId}/current", rs.getStudentCurrentVisit)
		r.With(tenant.RequiresPermission("attendance:read")).Get(routeGroupByGroupID, rs.getVisitsByGroup)

		// Write operations
		r.With(tenant.RequiresPermission("attendance:checkin")).Post("/", rs.createVisit)
		r.With(tenant.RequiresPermission("attendance:update")).Put("/{id}", rs.updateVisit)
		r.With(tenant.RequiresPermission("attendance:delete")).Delete("/{id}", rs.deleteVisit)
		r.With(tenant.RequiresPermission("attendance:checkout")).Post(routeEndByID, rs.endVisit)

		// Immediate checkout for students
		r.With(tenant.RequiresPermission("attendance:checkout")).Post("/student/{studentId}/checkout", rs.checkoutStudent)

		// Immediate check-in for students (from home)
		r.With(tenant.RequiresPermission("attendance:checkin")).Post("/student/{studentId}/checkin", rs.checkinStudent)
	})

	// Supervisors
	r.Route("/supervisors", func(r chi.Router) {
		// Read operations
		r.With(tenant.RequiresPermission("group:read")).Get("/", rs.listSupervisors)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}", rs.getSupervisor)
		r.With(tenant.RequiresPermission("group:read")).Get("/staff/{staffId}", rs.getStaffSupervisions)
		r.With(tenant.RequiresPermission("group:read")).Get("/staff/{staffId}/active", rs.getStaffActiveSupervisions)
		r.With(tenant.RequiresPermission("group:read")).Get(routeGroupByGroupID, rs.getSupervisorsByGroup)

		// Write operations
		r.With(tenant.RequiresPermission("group:assign")).Post("/", rs.createSupervisor)
		r.With(tenant.RequiresPermission("group:assign")).Put("/{id}", rs.updateSupervisor)
		r.With(tenant.RequiresPermission("group:assign")).Delete("/{id}", rs.deleteSupervisor)
		r.With(tenant.RequiresPermission("group:assign")).Post(routeEndByID, rs.endSupervision)
	})

	// Combined Groups
	r.Route("/combined", func(r chi.Router) {
		// Read operations
		r.With(tenant.RequiresPermission("group:read")).Get("/", rs.listCombinedGroups)
		r.With(tenant.RequiresPermission("group:read")).Get("/active", rs.getActiveCombinedGroups)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}", rs.getCombinedGroup)
		r.With(tenant.RequiresPermission("group:read")).Get("/{id}/groups", rs.getCombinedGroupGroups)

		// Write operations
		r.With(tenant.RequiresPermission("group:create")).Post("/", rs.createCombinedGroup)
		r.With(tenant.RequiresPermission("group:update")).Put("/{id}", rs.updateCombinedGroup)
		r.With(tenant.RequiresPermission("group:delete")).Delete("/{id}", rs.deleteCombinedGroup)
		r.With(tenant.RequiresPermission("group:update")).Post(routeEndByID, rs.endCombinedGroup)
	})

	// Group Mappings
	r.Route("/mappings", func(r chi.Router) {
		// Read operations
		r.With(tenant.RequiresPermission("group:read")).Get(routeGroupByGroupID, rs.getGroupMappings)
		r.With(tenant.RequiresPermission("group:read")).Get("/combined/{combinedId}", rs.getCombinedGroupMappings)

		// Write operations
		r.With(tenant.RequiresPermission("group:update")).Post("/add", rs.addGroupToCombination)
		r.With(tenant.RequiresPermission("group:update")).Post("/remove", rs.removeGroupFromCombination)
	})

	// Analytics
	r.Route("/analytics", func(r chi.Router) {
		r.With(tenant.RequiresPermission("group:read")).Get("/counts", rs.getCounts)
		r.With(tenant.RequiresPermission("group:read")).Get("/room/{roomId}/utilization", rs.getRoomUtilization)
		r.With(tenant.RequiresPermission("attendance:read")).Get("/student/{studentId}/attendance", rs.getStudentAttendance)
		r.With(tenant.RequiresPermission("group:read")).Get("/dashboard", rs.getDashboardAnalytics)
	})

	return r
}

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

// ===== Conversion Functions =====

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

// newVisitResponse converts a visit model to a response object
func newVisitResponse(visit *active.Visit) VisitResponse {
	response := VisitResponse{
		ID:            visit.ID,
		StudentID:     visit.StudentID,
		ActiveGroupID: visit.ActiveGroupID,
		CheckInTime:   visit.EntryTime,
		CheckOutTime:  visit.ExitTime,
		IsActive:      visit.IsActive(),
		CreatedAt:     visit.CreatedAt,
		UpdatedAt:     visit.UpdatedAt,
	}

	// Add related information if available
	if visit.Student != nil && visit.Student.Person != nil {
		response.StudentName = visit.Student.Person.GetFullName()
	}
	if visit.ActiveGroup != nil {
		response.ActiveGroupName = displayGroupPrefix + strconv.FormatInt(visit.ActiveGroup.GroupID, 10)
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

// ===== Active Group Handlers =====

// listActiveGroups handles listing all active groups
func (rs *Resource) listActiveGroups(w http.ResponseWriter, r *http.Request) {
	queryOptions := rs.parseActiveGroupQueryParams(r)

	groups, err := rs.ActiveService.ListActiveGroups(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	includeRelations := r.URL.Query().Get("active") == "true" || r.URL.Query().Get("is_active") == "true"
	if includeRelations && len(groups) > 0 {
		rs.loadActiveGroupRelations(r, groups)
	}

	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active groups retrieved successfully")
}

// parseActiveGroupQueryParams parses query parameters for active groups
func (rs *Resource) parseActiveGroupQueryParams(r *http.Request) *base.QueryOptions {
	queryOptions := base.NewQueryOptions()

	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		if isActive {
			queryOptions.Filter.IsNull("end_time")
		} else {
			queryOptions.Filter.IsNotNull("end_time")
		}
	}

	return queryOptions
}

// loadActiveGroupRelations loads rooms and supervisors for active groups
func (rs *Resource) loadActiveGroupRelations(r *http.Request, groups []*active.Group) {
	roomMap := rs.loadRoomsMap(r, groups)
	supervisorMap := rs.loadActiveSupervisorsMap(r, groups)

	for _, group := range groups {
		if supervisors, ok := supervisorMap[group.ID]; ok {
			group.Supervisors = supervisors
		}
		if room, ok := roomMap[group.RoomID]; ok {
			group.Room = room
		}
	}
}

// loadRoomsMap loads rooms and returns a map of room ID to room
func (rs *Resource) loadRoomsMap(r *http.Request, groups []*active.Group) map[int64]*facilities.Room {
	roomIDs := rs.collectUniqueRoomIDs(groups)
	roomMap := make(map[int64]*facilities.Room)

	if len(roomIDs) > 0 {
		var rooms []*facilities.Room
		err := rs.db.NewSelect().
			Model(&rooms).
			ModelTableExpr(`facilities.rooms AS "room"`).
			Where(`"room".id IN (?)`, bun.In(roomIDs)).
			Scan(r.Context())
		if err == nil {
			for _, room := range rooms {
				roomMap[room.ID] = room
			}
		}
	}

	return roomMap
}

// collectUniqueRoomIDs collects unique room IDs from groups
func (rs *Resource) collectUniqueRoomIDs(groups []*active.Group) []int64 {
	roomIDs := make([]int64, 0, len(groups))
	roomIDMap := make(map[int64]bool)

	for _, group := range groups {
		if group.RoomID > 0 && !roomIDMap[group.RoomID] {
			roomIDs = append(roomIDs, group.RoomID)
			roomIDMap[group.RoomID] = true
		}
	}

	return roomIDs
}

// loadActiveSupervisorsMap loads supervisors and returns a map of group ID to active supervisors
func (rs *Resource) loadActiveSupervisorsMap(r *http.Request, groups []*active.Group) map[int64][]*active.GroupSupervisor {
	groupIDs := make([]int64, len(groups))
	for i, group := range groups {
		groupIDs[i] = group.ID
	}

	allSupervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupIDs(r.Context(), groupIDs)
	if err != nil {
		log.Printf("Error loading supervisors: %v", err)
		return make(map[int64][]*active.GroupSupervisor)
	}

	activeSupervisors := make([]*active.GroupSupervisor, 0, len(allSupervisors))
	for _, supervisor := range allSupervisors {
		if supervisor.IsActive() {
			activeSupervisors = append(activeSupervisors, supervisor)
		}
	}

	supervisorMap := make(map[int64][]*active.GroupSupervisor)
	for _, supervisor := range activeSupervisors {
		supervisorMap[supervisor.GroupID] = append(supervisorMap[supervisor.GroupID], supervisor)
	}

	return supervisorMap
}

// getActiveGroup handles getting an active group by ID
func (rs *Resource) getActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Get active group
	group, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newActiveGroupResponse(group)

	common.Respond(w, r, http.StatusOK, response, "Active group retrieved successfully")
}

// getActiveGroupsByRoom handles getting active groups by room ID
func (rs *Resource) getActiveGroupsByRoom(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	roomID, err := common.ParseIDParam(r, "roomId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid room ID")))
		return
	}

	// Get active groups for room
	groups, err := rs.ActiveService.FindActiveGroupsByRoomID(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Room active groups retrieved successfully")
}

// getActiveGroupsByGroup handles getting active groups by group ID
func (rs *Resource) getActiveGroupsByGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get active groups for group
	groups, err := rs.ActiveService.FindActiveGroupsByGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group active sessions retrieved successfully")
}

// getActiveGroupVisits handles getting visits for an active group
func (rs *Resource) getActiveGroupVisits(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Get active group with visits
	group, err := rs.ActiveService.GetActiveGroupWithVisits(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(group.Visits))
	for _, visit := range group.Visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active group visits retrieved successfully")
}

// getActiveGroupVisitsWithDisplay handles getting visits with student display data in one query (optimized for SSE)
func (rs *Resource) getActiveGroupVisitsWithDisplay(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	staff, err := rs.extractStaffFromRequest(w, r)
	if err != nil {
		return
	}

	if rs.verifyStaffSupervisionAccess(w, r, staff.ID, id) != nil {
		return
	}

	results, err := rs.fetchVisitsWithDisplayData(r, id)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	responses := rs.buildVisitDisplayResponses(results)
	common.Respond(w, r, http.StatusOK, responses, "Active group visits with display data retrieved successfully")
}

// extractStaffFromRequest extracts staff information from tenant context
func (rs *Resource) extractStaffFromRequest(w http.ResponseWriter, r *http.Request) (*users.Staff, error) {
	tc := tenant.TenantFromCtx(r.Context())
	if tc == nil || tc.UserEmail == "" {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid session")))
		return nil, errors.New("invalid session")
	}

	account, err := rs.AuthService.GetAccountByEmail(r.Context(), tc.UserEmail)
	if err != nil || account == nil {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("account not found")))
		return nil, errors.New("account not found")
	}

	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err != nil || person == nil {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("person not found")))
		return nil, errors.New("person not found")
	}

	staff, err := rs.PersonService.StaffRepository().FindByPersonID(r.Context(), person.ID)
	if err != nil || staff == nil {
		common.RenderError(w, r, ErrorForbidden(errors.New("user is not a staff member")))
		return nil, errors.New("user is not a staff member")
	}

	return staff, nil
}

// verifyStaffSupervisionAccess verifies staff has permission to view an active group
func (rs *Resource) verifyStaffSupervisionAccess(w http.ResponseWriter, r *http.Request, staffID int64, activeGroupID int64) error {
	supervisions, err := rs.ActiveService.GetStaffActiveSupervisions(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return err
	}

	hasPermission := false
	for _, supervision := range supervisions {
		if supervision.GroupID == activeGroupID {
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		common.RenderError(w, r, ErrorForbidden(errors.New("not authorized to view this group")))
		return errors.New("not authorized")
	}

	_, err = rs.ActiveService.GetActiveGroup(r.Context(), activeGroupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return err
	}

	return nil
}

// visitWithStudent is a helper struct for the JOIN query
type visitWithStudent struct {
	VisitID       int64      `bun:"visit_id"`
	StudentID     int64      `bun:"student_id"`
	ActiveGroupID int64      `bun:"active_group_id"`
	EntryTime     time.Time  `bun:"entry_time"`
	ExitTime      *time.Time `bun:"exit_time"`
	FirstName     string     `bun:"first_name"`
	LastName      string     `bun:"last_name"`
	SchoolClass   string     `bun:"school_class"`
	OGSGroupName  string     `bun:"ogs_group_name"`
	CreatedAt     time.Time  `bun:"created_at"`
	UpdatedAt     time.Time  `bun:"updated_at"`
}

// fetchVisitsWithDisplayData fetches visits with student display data
func (rs *Resource) fetchVisitsWithDisplayData(r *http.Request, activeGroupID int64) ([]visitWithStudent, error) {
	var results []visitWithStudent
	err := rs.db.NewSelect().
		ColumnExpr("v.id AS visit_id").
		ColumnExpr("v.student_id").
		ColumnExpr("v.active_group_id").
		ColumnExpr("v.entry_time").
		ColumnExpr("v.exit_time").
		ColumnExpr("v.created_at").
		ColumnExpr("v.updated_at").
		ColumnExpr("p.first_name").
		ColumnExpr("p.last_name").
		ColumnExpr("COALESCE(s.school_class, '') AS school_class").
		ColumnExpr("COALESCE(g.name, '') AS ogs_group_name").
		TableExpr("active.visits AS v").
		Join("INNER JOIN users.students AS s ON s.id = v.student_id").
		Join("INNER JOIN users.persons AS p ON p.id = s.person_id").
		Join("LEFT JOIN education.groups AS g ON g.id = s.group_id").
		Where("v.active_group_id = ?", activeGroupID).
		Where("v.exit_time IS NULL").
		OrderExpr("v.entry_time DESC").
		Scan(r.Context(), &results)

	return results, err
}

// buildVisitDisplayResponses builds visit responses with display data
func (rs *Resource) buildVisitDisplayResponses(results []visitWithStudent) []VisitWithDisplayDataResponse {
	responses := make([]VisitWithDisplayDataResponse, 0, len(results))
	for _, result := range results {
		studentName := result.FirstName + " " + result.LastName
		responses = append(responses, VisitWithDisplayDataResponse{
			ID:            result.VisitID,
			StudentID:     result.StudentID,
			ActiveGroupID: result.ActiveGroupID,
			CheckInTime:   result.EntryTime,
			CheckOutTime:  result.ExitTime,
			IsActive:      result.ExitTime == nil,
			StudentName:   studentName,
			SchoolClass:   result.SchoolClass,
			GroupName:     result.OGSGroupName,
			CreatedAt:     result.CreatedAt,
			UpdatedAt:     result.UpdatedAt,
		})
	}
	return responses
}

// getActiveGroupSupervisors handles getting supervisors for an active group
func (rs *Resource) getActiveGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Get active group with supervisors
	group, err := rs.ActiveService.GetActiveGroupWithSupervisors(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(group.Supervisors))
	for _, supervisor := range group.Supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active group supervisors retrieved successfully")
}

// createActiveGroup handles creating a new active group
func (rs *Resource) createActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &ActiveGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create active group
	group := &active.Group{
		GroupID:   req.GroupID,
		RoomID:    req.RoomID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	// Create active group
	if err := rs.ActiveService.CreateActiveGroup(r.Context(), group); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created active group
	createdGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), group.ID)
	if err != nil {
		// Still return success but with the basic group info
		response := newActiveGroupResponse(group)
		common.Respond(w, r, http.StatusCreated, response, "Active group created successfully")
		return
	}

	// Return the active group with all details
	response := newActiveGroupResponse(createdGroup)
	common.Respond(w, r, http.StatusCreated, response, "Active group created successfully")
}

// updateActiveGroup handles updating an active group
func (rs *Resource) updateActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Parse request
	req := &ActiveGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing active group
	existing, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.GroupID = req.GroupID
	existing.RoomID = req.RoomID
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update active group
	if err := rs.ActiveService.UpdateActiveGroup(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated active group
	updatedGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		// Still return success but with the basic group info
		response := newActiveGroupResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Active group updated successfully")
		return
	}

	// Return the updated active group with all details
	response := newActiveGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Active group updated successfully")
}

// deleteActiveGroup handles deleting an active group
func (rs *Resource) deleteActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// Delete active group
	if err := rs.ActiveService.DeleteActiveGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Active group deleted successfully")
}

// endActiveGroup handles ending an active group session
func (rs *Resource) endActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidActiveGroupID)))
		return
	}

	// End active group session
	if err := rs.ActiveService.EndActiveGroupSession(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated active group
	updatedGroup, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Active group session ended successfully")
		return
	}

	// Return the updated active group
	response := newActiveGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Active group session ended successfully")
}

// ===== Visit Handlers =====

// listVisits handles listing all visits
func (rs *Resource) listVisits(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Set table alias to match repository implementation
	queryOptions.Filter.WithTableAlias("visit")

	// Get active status filter
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		if isActive {
			// For active visits, exit_time should be NULL
			queryOptions.Filter.IsNull("exit_time")
		} else {
			// For inactive visits, exit_time should NOT be NULL
			queryOptions.Filter.IsNotNull("exit_time")
		}
	}

	// Get visits
	visits, err := rs.ActiveService.ListVisits(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Visits retrieved successfully")
}

// getVisit handles getting a visit by ID
func (rs *Resource) getVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Get visit
	visit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newVisitResponse(visit)

	common.Respond(w, r, http.StatusOK, response, "Visit retrieved successfully")
}

// getStudentVisits handles getting visits for a student
func (rs *Resource) getStudentVisits(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStudentID)))
		return
	}

	// Get visits for student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Student visits retrieved successfully")
}

// getStudentCurrentVisit handles getting the current active visit for a student
func (rs *Resource) getStudentCurrentVisit(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStudentID)))
		return
	}

	// Get current visit for student
	visit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Check if student has an active visit
	if visit == nil {
		common.Respond(w, r, http.StatusOK, nil, "Student has no active visit")
		return
	}

	// Prepare response
	response := newVisitResponse(visit)

	common.Respond(w, r, http.StatusOK, response, "Student current visit retrieved successfully")
}

// getVisitsByGroup handles getting visits for an active group
func (rs *Resource) getVisitsByGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get visits for active group
	visits, err := rs.ActiveService.FindVisitsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(visits))
	for _, visit := range visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group visits retrieved successfully")
}

// createVisit handles creating a new visit
func (rs *Resource) createVisit(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create visit
	visit := &active.Visit{
		StudentID:     req.StudentID,
		ActiveGroupID: req.ActiveGroupID,
		EntryTime:     req.CheckInTime,
		ExitTime:      req.CheckOutTime,
	}

	// Create visit
	if err := rs.ActiveService.CreateVisit(r.Context(), visit); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created visit
	createdVisit, err := rs.ActiveService.GetVisit(r.Context(), visit.ID)
	if err != nil {
		// Still return success but with the basic visit info
		response := newVisitResponse(visit)
		common.Respond(w, r, http.StatusCreated, response, "Visit created successfully")
		return
	}

	// Return the visit with all details
	response := newVisitResponse(createdVisit)
	common.Respond(w, r, http.StatusCreated, response, "Visit created successfully")
}

// updateVisit handles updating a visit
func (rs *Resource) updateVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing visit
	existing, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StudentID = req.StudentID
	existing.ActiveGroupID = req.ActiveGroupID
	existing.EntryTime = req.CheckInTime
	existing.ExitTime = req.CheckOutTime

	// Update visit
	if err := rs.ActiveService.UpdateVisit(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated visit
	updatedVisit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		// Still return success but with the basic visit info
		response := newVisitResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Visit updated successfully")
		return
	}

	// Return the updated visit with all details
	response := newVisitResponse(updatedVisit)
	common.Respond(w, r, http.StatusOK, response, "Visit updated successfully")
}

// deleteVisit handles deleting a visit
func (rs *Resource) deleteVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// Delete visit
	if err := rs.ActiveService.DeleteVisit(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Visit deleted successfully")
}

// endVisit handles ending a visit
func (rs *Resource) endVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidVisitID)))
		return
	}

	// End visit
	if err := rs.ActiveService.EndVisit(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated visit
	updatedVisit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Visit ended successfully")
		return
	}

	// Return the updated visit
	response := newVisitResponse(updatedVisit)
	common.Respond(w, r, http.StatusOK, response, "Visit ended successfully")
}

// ===== Supervisor Handlers =====

// listSupervisors handles listing all group supervisors
func (rs *Resource) listSupervisors(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Get active status filter
	// Note: active.group_supervisors doesn't have is_active column, use "active_only" filter
	// which the service/repository interprets as end_date IS NULL OR end_date > NOW()
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("active_only", isActive)
	}

	// Get supervisors
	supervisors, err := rs.ActiveService.ListGroupSupervisors(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Supervisors retrieved successfully")
}

// getSupervisor handles getting a group supervisor by ID
func (rs *Resource) getSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// Get supervisor
	supervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newSupervisorResponse(supervisor)

	common.Respond(w, r, http.StatusOK, response, "Supervisor retrieved successfully")
}

// getStaffSupervisions handles getting supervisions for a staff member
func (rs *Resource) getStaffSupervisions(w http.ResponseWriter, r *http.Request) {
	// Parse staff ID from URL
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Get supervisions for staff
	supervisors, err := rs.ActiveService.FindSupervisorsByStaffID(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff supervisions retrieved successfully")
}

// getStaffActiveSupervisions handles getting active supervisions for a staff member
func (rs *Resource) getStaffActiveSupervisions(w http.ResponseWriter, r *http.Request) {
	// Parse staff ID from URL
	staffID, err := common.ParseIDParam(r, "staffId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid staff ID")))
		return
	}

	// Get active supervisions for staff
	supervisors, err := rs.ActiveService.GetStaffActiveSupervisions(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Staff active supervisions retrieved successfully")
}

// getSupervisorsByGroup handles getting supervisors for an active group
func (rs *Resource) getSupervisorsByGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get supervisors for active group
	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]SupervisorResponse, 0, len(supervisors))
	for _, supervisor := range supervisors {
		responses = append(responses, newSupervisorResponse(supervisor))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group supervisors retrieved successfully")
}

// createSupervisor handles creating a new group supervisor
func (rs *Resource) createSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &SupervisorRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create supervisor
	supervisor := &active.GroupSupervisor{
		StaffID:   req.StaffID,
		GroupID:   req.ActiveGroupID,
		Role:      "Supervisor", // Default role
		StartDate: req.StartTime,
		EndDate:   req.EndTime,
	}

	// Create supervisor
	if err := rs.ActiveService.CreateGroupSupervisor(r.Context(), supervisor); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the created supervisor
	createdSupervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), supervisor.ID)
	if err != nil {
		// Still return success but with the basic supervisor info
		response := newSupervisorResponse(supervisor)
		common.Respond(w, r, http.StatusCreated, response, "Supervisor created successfully")
		return
	}

	// Return the supervisor with all details
	response := newSupervisorResponse(createdSupervisor)
	common.Respond(w, r, http.StatusCreated, response, "Supervisor created successfully")
}

// updateSupervisor handles updating a group supervisor
func (rs *Resource) updateSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// Parse request
	req := &SupervisorRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing supervisor
	existing, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StaffID = req.StaffID
	existing.GroupID = req.ActiveGroupID
	existing.StartDate = req.StartTime
	existing.EndDate = req.EndTime

	// Update supervisor
	if err := rs.ActiveService.UpdateGroupSupervisor(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		// Still return success but with the basic supervisor info
		response := newSupervisorResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Supervisor updated successfully")
		return
	}

	// Return the updated supervisor with all details
	response := newSupervisorResponse(updatedSupervisor)
	common.Respond(w, r, http.StatusOK, response, "Supervisor updated successfully")
}

// deleteSupervisor handles deleting a group supervisor
func (rs *Resource) deleteSupervisor(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// Delete supervisor
	if err := rs.ActiveService.DeleteGroupSupervisor(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Supervisor deleted successfully")
}

// endSupervision handles ending a supervision
func (rs *Resource) endSupervision(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidSupervisorID)))
		return
	}

	// End supervision
	if err := rs.ActiveService.EndSupervision(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated supervisor
	updatedSupervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Supervision ended successfully")
		return
	}

	// Return the updated supervisor
	response := newSupervisorResponse(updatedSupervisor)
	common.Respond(w, r, http.StatusOK, response, "Supervision ended successfully")
}

// ===== Combined Group Handlers =====

// listCombinedGroups handles listing all combined groups
func (rs *Resource) listCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Get active status filter
	// Note: active.combined_groups doesn't have is_active column, use "active_only" filter
	// which the service/repository interprets as end_time IS NULL OR end_time > NOW()
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("active_only", isActive)
	}

	// Get combined groups
	groups, err := rs.ActiveService.ListCombinedGroups(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]CombinedGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newCombinedGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined groups retrieved successfully")
}

// getActiveCombinedGroups handles getting all active combined groups
func (rs *Resource) getActiveCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get active combined groups
	groups, err := rs.ActiveService.FindActiveCombinedGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	responses := make([]CombinedGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newCombinedGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active combined groups retrieved successfully")
}

// getCombinedGroup handles getting a combined group by ID
func (rs *Resource) getCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Get combined group
	group, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Prepare response
	response := newCombinedGroupResponse(group)

	common.Respond(w, r, http.StatusOK, response, "Combined group retrieved successfully")
}

// getCombinedGroupGroups handles getting active groups in a combined group
func (rs *Resource) getCombinedGroupGroups(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Get combined group with groups
	combinedGroup, err := rs.ActiveService.GetCombinedGroupWithGroups(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(combinedGroup.ActiveGroups))
	for _, group := range combinedGroup.ActiveGroups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined group's active groups retrieved successfully")
}

// createCombinedGroup handles creating a new combined group
func (rs *Resource) createCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &CombinedGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create combined group
	group := &active.CombinedGroup{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	// Create combined group
	if err := rs.ActiveService.CreateCombinedGroup(r.Context(), group); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Add groups to the combined group if provided
	if len(req.GroupIDs) > 0 {
		for _, groupID := range req.GroupIDs {
			if rs.ActiveService.AddGroupToCombination(r.Context(), group.ID, groupID) != nil {
				// Log error but continue (see #554 for partial failure handling)
				continue
			}
		}
	}

	// Get the created combined group with all groups
	createdGroup, err := rs.ActiveService.GetCombinedGroupWithGroups(r.Context(), group.ID)
	if err != nil {
		// Still return success but with the basic group info
		response := newCombinedGroupResponse(group)
		common.Respond(w, r, http.StatusCreated, response, "Combined group created successfully")
		return
	}

	// Return the combined group with all details
	response := newCombinedGroupResponse(createdGroup)
	common.Respond(w, r, http.StatusCreated, response, "Combined group created successfully")
}

// updateCombinedGroup handles updating a combined group
func (rs *Resource) updateCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Parse request
	req := &CombinedGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing combined group
	existing, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Update fields
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update combined group
	if err := rs.ActiveService.UpdateCombinedGroup(r.Context(), existing); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated combined group
	updatedGroup, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		// Still return success but with the basic group info
		response := newCombinedGroupResponse(existing)
		common.Respond(w, r, http.StatusOK, response, "Combined group updated successfully")
		return
	}

	// Return the updated combined group with all details
	response := newCombinedGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Combined group updated successfully")
}

// deleteCombinedGroup handles deleting a combined group
func (rs *Resource) deleteCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Delete combined group
	if err := rs.ActiveService.DeleteCombinedGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Combined group deleted successfully")
}

// endCombinedGroup handles ending a combined group
func (rs *Resource) endCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// End combined group
	if err := rs.ActiveService.EndCombinedGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the updated combined group
	updatedGroup, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Combined group ended successfully")
		return
	}

	// Return the updated combined group
	response := newCombinedGroupResponse(updatedGroup)
	common.Respond(w, r, http.StatusOK, response, "Combined group ended successfully")
}

// ===== Group Mapping Handlers =====

// getGroupMappings handles getting mappings for an active group
func (rs *Resource) getGroupMappings(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := common.ParseIDParam(r, "groupId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidGroupID)))
		return
	}

	// Get mappings for active group
	mappings, err := rs.ActiveService.GetGroupMappingsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]GroupMappingResponse, 0, len(mappings))
	for _, mapping := range mappings {
		responses = append(responses, newGroupMappingResponse(mapping))
	}

	common.Respond(w, r, http.StatusOK, responses, "Group mappings retrieved successfully")
}

// getCombinedGroupMappings handles getting mappings for a combined group
func (rs *Resource) getCombinedGroupMappings(w http.ResponseWriter, r *http.Request) {
	// Parse combined group ID from URL
	combinedID, err := common.ParseIDParam(r, "combinedId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidCombinedGroupID)))
		return
	}

	// Get mappings for combined group
	mappings, err := rs.ActiveService.GetGroupMappingsByCombinedGroupID(r.Context(), combinedID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	responses := make([]GroupMappingResponse, 0, len(mappings))
	for _, mapping := range mappings {
		responses = append(responses, newGroupMappingResponse(mapping))
	}

	common.Respond(w, r, http.StatusOK, responses, "Combined group mappings retrieved successfully")
}

// addGroupToCombination handles adding an active group to a combined group
func (rs *Resource) addGroupToCombination(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupMappingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Add group to combination
	if err := rs.ActiveService.AddGroupToCombination(r.Context(), req.CombinedGroupID, req.ActiveGroupID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Get the mappings for verification
	mappings, err := rs.ActiveService.GetGroupMappingsByCombinedGroupID(r.Context(), req.CombinedGroupID)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, msgGroupAddedToCombination)
		return
	}

	// Find the newly created mapping
	var newMapping *active.GroupMapping
	for _, mapping := range mappings {
		if mapping.ActiveGroupID == req.ActiveGroupID {
			newMapping = mapping
			break
		}
	}

	if newMapping == nil {
		common.Respond(w, r, http.StatusOK, nil, msgGroupAddedToCombination)
		return
	}

	// Return the mapping
	response := newGroupMappingResponse(newMapping)
	common.Respond(w, r, http.StatusOK, response, msgGroupAddedToCombination)
}

// removeGroupFromCombination handles removing an active group from a combined group
func (rs *Resource) removeGroupFromCombination(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupMappingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Remove group from combination
	if err := rs.ActiveService.RemoveGroupFromCombination(r.Context(), req.CombinedGroupID, req.ActiveGroupID); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Group removed from combination successfully")
}

// ===== Analytics Handlers =====

// getCounts handles getting various counts for analytics
func (rs *Resource) getCounts(w http.ResponseWriter, r *http.Request) {
	// Get active groups count
	activeGroupsCount, err := rs.ActiveService.GetActiveGroupsCount(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get total visits count
	totalVisitsCount, err := rs.ActiveService.GetTotalVisitsCount(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Get active visits count
	activeVisitsCount, err := rs.ActiveService.GetActiveVisitsCount(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	response := AnalyticsResponse{
		ActiveGroupsCount: activeGroupsCount,
		TotalVisitsCount:  totalVisitsCount,
		ActiveVisitsCount: activeVisitsCount,
	}

	common.Respond(w, r, http.StatusOK, response, "Counts retrieved successfully")
}

// getRoomUtilization handles getting room utilization for analytics
func (rs *Resource) getRoomUtilization(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	roomID, err := common.ParseIDParam(r, "roomId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("invalid room ID")))
		return
	}

	// Get room utilization
	utilization, err := rs.ActiveService.GetRoomUtilization(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	response := AnalyticsResponse{
		RoomUtilization: utilization,
	}

	common.Respond(w, r, http.StatusOK, response, "Room utilization retrieved successfully")
}

// getStudentAttendance handles getting student attendance rate for analytics
func (rs *Resource) getStudentAttendance(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsgInvalidStudentID)))
		return
	}

	// Get student attendance rate
	attendanceRate, err := rs.ActiveService.GetStudentAttendanceRate(r.Context(), studentID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Build response
	response := AnalyticsResponse{
		AttendanceRate: attendanceRate,
	}

	common.Respond(w, r, http.StatusOK, response, "Student attendance rate retrieved successfully")
}

// getDashboardAnalytics handles getting dashboard analytics data
func (rs *Resource) getDashboardAnalytics(w http.ResponseWriter, r *http.Request) {
	// Get dashboard analytics
	analytics, err := rs.ActiveService.GetDashboardAnalytics(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	response := DashboardAnalyticsResponse{
		StudentsPresent:      analytics.StudentsPresent,
		StudentsInTransit:    analytics.StudentsInTransit,
		StudentsOnPlayground: analytics.StudentsOnPlayground,
		StudentsInRooms:      analytics.StudentsInRooms,
		ActiveActivities:     analytics.ActiveActivities,
		FreeRooms:            analytics.FreeRooms,
		TotalRooms:           analytics.TotalRooms,
		CapacityUtilization:  analytics.CapacityUtilization,
		ActivityCategories:   analytics.ActivityCategories,
		ActiveOGSGroups:      analytics.ActiveOGSGroups,
		StudentsInGroupRooms: analytics.StudentsInGroupRooms,
		SupervisorsToday:     analytics.SupervisorsToday,
		StudentsInHomeRoom:   analytics.StudentsInHomeRoom,
		RecentActivity:       make([]RecentActivityItem, 0),
		CurrentActivities:    make([]CurrentActivityItem, 0),
		ActiveGroupsSummary:  make([]ActiveGroupSummary, 0),
		LastUpdated:          time.Now(),
	}

	// Map recent activity
	for _, activity := range analytics.RecentActivity {
		response.RecentActivity = append(response.RecentActivity, RecentActivityItem{
			Type:      activity.Type,
			GroupName: activity.GroupName,
			RoomName:  activity.RoomName,
			Count:     activity.Count,
			Timestamp: activity.Timestamp,
		})
	}

	// Map current activities
	for _, activity := range analytics.CurrentActivities {
		response.CurrentActivities = append(response.CurrentActivities, CurrentActivityItem{
			Name:         activity.Name,
			Category:     activity.Category,
			Participants: activity.Participants,
			MaxCapacity:  activity.MaxCapacity,
			Status:       activity.Status,
		})
	}

	// Map active groups summary
	for _, group := range analytics.ActiveGroupsSummary {
		response.ActiveGroupsSummary = append(response.ActiveGroupsSummary, ActiveGroupSummary{
			Name:         group.Name,
			Type:         group.Type,
			StudentCount: group.StudentCount,
			Location:     group.Location,
			Status:       group.Status,
		})
	}

	common.Respond(w, r, http.StatusOK, response, "Dashboard analytics retrieved successfully")
}

// ======== Unclaimed Groups Management (Deviceless Claiming) ========

// listUnclaimedGroups returns all active groups that have no supervisors
// This is used for deviceless rooms like Schulhof where teachers claim via frontend
func (rs *Resource) listUnclaimedGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := rs.ActiveService.GetUnclaimedActiveGroups(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, groups, "Unclaimed groups retrieved successfully")
}

// claimGroup allows authenticated staff to claim supervision of an active group
func (rs *Resource) claimGroup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get group ID from URL
	groupIDStr := chi.URLParam(r, "id")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		common.RespondWithError(w, r, http.StatusBadRequest, "Invalid group ID")
		return
	}

	// Get authenticated user from tenant context
	tc := tenant.TenantFromCtx(ctx)
	if tc == nil || tc.UserEmail == "" {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Invalid session")
		return
	}

	// Get account by email
	account, err := rs.AuthService.GetAccountByEmail(ctx, tc.UserEmail)
	if err != nil || account == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Account not found")
		return
	}

	// Get person from account ID
	person, err := rs.PersonService.FindByAccountID(ctx, int64(account.ID))
	if err != nil || person == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Person not found")
		return
	}

	// Get staff record from person
	staff, err := rs.PersonService.StaffRepository().FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		common.RespondWithError(w, r, http.StatusUnauthorized, "Staff authentication required")
		return
	}

	// Claim the group (default role: "supervisor")
	supervisor, err := rs.ActiveService.ClaimActiveGroup(ctx, groupID, staff.ID, "supervisor")
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, supervisor, "Successfully claimed supervision")
}

// ============================================================================
// EXPORTED HANDLER METHODS (for testing)
// ============================================================================

// Active Group Handlers
func (rs *Resource) ListActiveGroupsHandler() http.HandlerFunc { return rs.listActiveGroups }
func (rs *Resource) GetActiveGroupHandler() http.HandlerFunc   { return rs.getActiveGroup }
func (rs *Resource) CreateActiveGroupHandler() http.HandlerFunc {
	return rs.createActiveGroup
}
func (rs *Resource) UpdateActiveGroupHandler() http.HandlerFunc {
	return rs.updateActiveGroup
}
func (rs *Resource) DeleteActiveGroupHandler() http.HandlerFunc {
	return rs.deleteActiveGroup
}
func (rs *Resource) EndActiveGroupHandler() http.HandlerFunc { return rs.endActiveGroup }

// Visit Handlers
func (rs *Resource) ListVisitsHandler() http.HandlerFunc  { return rs.listVisits }
func (rs *Resource) GetVisitHandler() http.HandlerFunc    { return rs.getVisit }
func (rs *Resource) CreateVisitHandler() http.HandlerFunc { return rs.createVisit }
func (rs *Resource) UpdateVisitHandler() http.HandlerFunc { return rs.updateVisit }
func (rs *Resource) DeleteVisitHandler() http.HandlerFunc { return rs.deleteVisit }
func (rs *Resource) EndVisitHandler() http.HandlerFunc    { return rs.endVisit }
func (rs *Resource) GetStudentVisitsHandler() http.HandlerFunc {
	return rs.getStudentVisits
}
func (rs *Resource) GetStudentCurrentVisitHandler() http.HandlerFunc {
	return rs.getStudentCurrentVisit
}

// Supervisor Handlers
func (rs *Resource) ListSupervisorsHandler() http.HandlerFunc  { return rs.listSupervisors }
func (rs *Resource) GetSupervisorHandler() http.HandlerFunc    { return rs.getSupervisor }
func (rs *Resource) CreateSupervisorHandler() http.HandlerFunc { return rs.createSupervisor }
func (rs *Resource) UpdateSupervisorHandler() http.HandlerFunc { return rs.updateSupervisor }
func (rs *Resource) DeleteSupervisorHandler() http.HandlerFunc { return rs.deleteSupervisor }
func (rs *Resource) EndSupervisionHandler() http.HandlerFunc   { return rs.endSupervision }
func (rs *Resource) GetStaffSupervisionsHandler() http.HandlerFunc {
	return rs.getStaffSupervisions
}
func (rs *Resource) GetStaffActiveSupervisionsHandler() http.HandlerFunc {
	return rs.getStaffActiveSupervisions
}

// Analytics Handlers
func (rs *Resource) GetCountsHandler() http.HandlerFunc { return rs.getCounts }
func (rs *Resource) GetDashboardAnalyticsHandler() http.HandlerFunc {
	return rs.getDashboardAnalytics
}

func (rs *Resource) GetRoomUtilizationHandler() http.HandlerFunc { return rs.getRoomUtilization }
func (rs *Resource) GetStudentAttendanceHandler() http.HandlerFunc {
	return rs.getStudentAttendance
}

// Combined Group Handlers
func (rs *Resource) ListCombinedGroupsHandler() http.HandlerFunc  { return rs.listCombinedGroups }
func (rs *Resource) GetCombinedGroupHandler() http.HandlerFunc    { return rs.getCombinedGroup }
func (rs *Resource) CreateCombinedGroupHandler() http.HandlerFunc { return rs.createCombinedGroup }
func (rs *Resource) UpdateCombinedGroupHandler() http.HandlerFunc { return rs.updateCombinedGroup }
func (rs *Resource) DeleteCombinedGroupHandler() http.HandlerFunc { return rs.deleteCombinedGroup }
func (rs *Resource) EndCombinedGroupHandler() http.HandlerFunc    { return rs.endCombinedGroup }
func (rs *Resource) GetActiveCombinedGroupsHandler() http.HandlerFunc {
	return rs.getActiveCombinedGroups
}

// Group by filters Handlers
func (rs *Resource) GetActiveGroupsByRoomHandler() http.HandlerFunc {
	return rs.getActiveGroupsByRoom
}
func (rs *Resource) GetActiveGroupsByGroupHandler() http.HandlerFunc {
	return rs.getActiveGroupsByGroup
}
func (rs *Resource) GetActiveGroupVisitsHandler() http.HandlerFunc {
	return rs.getActiveGroupVisits
}
func (rs *Resource) GetActiveGroupVisitsWithDisplayHandler() http.HandlerFunc {
	return rs.getActiveGroupVisitsWithDisplay
}
func (rs *Resource) GetActiveGroupSupervisorsHandler() http.HandlerFunc {
	return rs.getActiveGroupSupervisors
}
func (rs *Resource) GetVisitsByGroupHandler() http.HandlerFunc {
	return rs.getVisitsByGroup
}
func (rs *Resource) GetSupervisorsByGroupHandler() http.HandlerFunc {
	return rs.getSupervisorsByGroup
}

// Group Mapping Handlers
func (rs *Resource) GetGroupMappingsHandler() http.HandlerFunc { return rs.getGroupMappings }
func (rs *Resource) GetCombinedGroupMappingsHandler() http.HandlerFunc {
	return rs.getCombinedGroupMappings
}
func (rs *Resource) AddGroupToCombinationHandler() http.HandlerFunc {
	return rs.addGroupToCombination
}
func (rs *Resource) RemoveGroupFromCombinationHandler() http.HandlerFunc {
	return rs.removeGroupFromCombination
}

// Unclaimed Group Handlers
func (rs *Resource) ListUnclaimedGroupsHandler() http.HandlerFunc { return rs.listUnclaimedGroups }
func (rs *Resource) ClaimGroupHandler() http.HandlerFunc          { return rs.claimGroup }

// Checkout Handler
func (rs *Resource) CheckoutStudentHandler() http.HandlerFunc { return rs.checkoutStudent }
