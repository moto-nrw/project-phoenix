package active

import (
	"log"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// Resource defines the active API resource
type Resource struct {
	ActiveService activeSvc.Service
}

// NewResource creates a new active resource
func NewResource(activeService activeSvc.Service) *Resource {
	return &Resource{
		ActiveService: activeService,
	}
}

// Router returns a configured router for active endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Active Groups
		r.Route("/groups", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listActiveGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/room/{roomId}", rs.getActiveGroupsByRoom)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", rs.getActiveGroupsByGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/visits", rs.getActiveGroupVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/supervisors", rs.getActiveGroupSupervisors)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteActiveGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", rs.endActiveGroup)
		})

		// Visits
		r.Route("/visits", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listVisits)
			r.With(authorize.GetResourceAuthorizer().RequiresResourceAccess("visit", policy.ActionView, VisitIDExtractor())).Get("/{id}", rs.getVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}", rs.getStudentVisits)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/current", rs.getStudentCurrentVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", rs.getVisitsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteVisit)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", rs.endVisit)
		})

		// Supervisors
		r.Route("/supervisors", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listSupervisors)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}", rs.getStaffSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/staff/{staffId}/active", rs.getStaffActiveSupervisions)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", rs.getSupervisorsByGroup)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/", rs.createSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Put("/{id}", rs.updateSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Delete("/{id}", rs.deleteSupervisor)
			r.With(authorize.RequiresPermission(permissions.GroupsAssign)).Post("/{id}/end", rs.endSupervision)
		})

		// Combined Groups
		r.Route("/combined", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listCombinedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/active", rs.getActiveCombinedGroups)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/groups", rs.getCombinedGroupGroups)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteCombinedGroup)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/{id}/end", rs.endCombinedGroup)
		})

		// Group Mappings
		r.Route("/mappings", func(r chi.Router) {
			// Read operations
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/group/{groupId}", rs.getGroupMappings)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/combined/{combinedId}", rs.getCombinedGroupMappings)

			// Write operations
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/add", rs.addGroupToCombination)
			r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Post("/remove", rs.removeGroupFromCombination)
		})

		// Analytics
		r.Route("/analytics", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/counts", rs.getCounts)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/room/{roomId}/utilization", rs.getRoomUtilization)
			r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/student/{studentId}/attendance", rs.getStudentAttendance)
		})
	})

	return r
}

// VisitIDExtractor extracts visit information for authorization
func VisitIDExtractor() authorize.ResourceExtractor {
	return func(r *http.Request) (interface{}, map[string]interface{}) {
		idStr := chi.URLParam(r, "id")
		if idStr == "" {
			return nil, nil
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, nil
		}

		// Return the visit ID as the resource ID
		return id, nil
	}
}

// ===== Response Types =====

// ActiveGroupResponse represents an active group API response
type ActiveGroupResponse struct {
	ID              int64      `json:"id"`
	GroupID         int64      `json:"group_id"`
	RoomID          int64      `json:"room_id"`
	StartTime       time.Time  `json:"start_time"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	IsActive        bool       `json:"is_active"`
	Notes           string     `json:"notes,omitempty"`
	VisitCount      int        `json:"visit_count,omitempty"`
	SupervisorCount int        `json:"supervisor_count,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
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
func (req *ActiveGroupRequest) Bind(r *http.Request) error {
	if req.GroupID <= 0 {
		return errors.New("group ID is required")
	}
	if req.RoomID <= 0 {
		return errors.New("room ID is required")
	}
	if req.StartTime.IsZero() {
		return errors.New("start time is required")
	}
	return nil
}

// Bind validates the visit request
func (req *VisitRequest) Bind(r *http.Request) error {
	if req.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if req.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
	}
	if req.CheckInTime.IsZero() {
		return errors.New("check-in time is required")
	}
	return nil
}

// Bind validates the supervisor request
func (req *SupervisorRequest) Bind(r *http.Request) error {
	if req.StaffID <= 0 {
		return errors.New("staff ID is required")
	}
	if req.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
	}
	if req.StartTime.IsZero() {
		return errors.New("start time is required")
	}
	return nil
}

// Bind validates the combined group request
func (req *CombinedGroupRequest) Bind(r *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if req.RoomID <= 0 {
		return errors.New("room ID is required")
	}
	if req.StartTime.IsZero() {
		return errors.New("start time is required")
	}
	return nil
}

// Bind validates the group mapping request
func (req *GroupMappingRequest) Bind(r *http.Request) error {
	if req.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
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
		response.SupervisorCount = len(group.Supervisors)
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
		response.ActiveGroupName = "Group #" + strconv.FormatInt(visit.ActiveGroup.GroupID, 10)
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
		response.ActiveGroupName = "Group #" + strconv.FormatInt(supervisor.ActiveGroup.GroupID, 10)
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
		response.GroupName = "Group #" + strconv.FormatInt(mapping.ActiveGroup.GroupID, 10)
	}
	if mapping.CombinedGroup != nil {
		response.CombinedName = "Combined Group #" + strconv.FormatInt(mapping.CombinedGroup.ID, 10)
	}

	return response
}

// ===== Active Group Handlers =====

// listActiveGroups handles listing all active groups
func (rs *Resource) listActiveGroups(w http.ResponseWriter, r *http.Request) {
	// Get query parameters
	queryOptions := base.NewQueryOptions()

	// Get active status filter
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("is_active", isActive)
	}

	// Get active groups
	groups, err := rs.ActiveService.ListActiveGroups(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]ActiveGroupResponse, 0, len(groups))
	for _, group := range groups {
		responses = append(responses, newActiveGroupResponse(group))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active groups retrieved successfully")
}

// getActiveGroup handles getting an active group by ID
func (rs *Resource) getActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid active group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active group
	group, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Prepare response
	response := newActiveGroupResponse(group)

	common.Respond(w, r, http.StatusOK, response, "Active group retrieved successfully")
}

// getActiveGroupsByRoom handles getting active groups by room ID
func (rs *Resource) getActiveGroupsByRoom(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	roomID, err := strconv.ParseInt(chi.URLParam(r, "roomId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active groups for room
	groups, err := rs.ActiveService.FindActiveGroupsByRoomID(r.Context(), roomID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active groups for group
	groups, err := rs.ActiveService.FindActiveGroupsByGroupID(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid active group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active group with visits
	group, err := rs.ActiveService.GetActiveGroupWithVisits(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]VisitResponse, 0, len(group.Visits))
	for _, visit := range group.Visits {
		responses = append(responses, newVisitResponse(visit))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active group visits retrieved successfully")
}

// getActiveGroupSupervisors handles getting supervisors for an active group
func (rs *Resource) getActiveGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid active group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active group with supervisors
	group, err := rs.ActiveService.GetActiveGroupWithSupervisors(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid active group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &ActiveGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing active group
	existing, err := rs.ActiveService.GetActiveGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	existing.GroupID = req.GroupID
	existing.RoomID = req.RoomID
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update active group
	if err := rs.ActiveService.UpdateActiveGroup(r.Context(), existing); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid active group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete active group
	if err := rs.ActiveService.DeleteActiveGroup(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Active group deleted successfully")
}

// endActiveGroup handles ending an active group session
func (rs *Resource) endActiveGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid active group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// End active group session
	if err := rs.ActiveService.EndActiveGroupSession(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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

	// Get active status filter
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("is_active", isActive)
	}

	// Get visits
	visits, err := rs.ActiveService.ListVisits(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid visit ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get visit
	visit, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Prepare response
	response := newVisitResponse(visit)

	common.Respond(w, r, http.StatusOK, response, "Visit retrieved successfully")
}

// getStudentVisits handles getting visits for a student
func (rs *Resource) getStudentVisits(w http.ResponseWriter, r *http.Request) {
	// Parse student ID from URL
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get visits for student
	visits, err := rs.ActiveService.FindVisitsByStudentID(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current visit for student
	visit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get visits for active group
	visits, err := rs.ActiveService.FindVisitsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid visit ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &VisitRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing visit
	existing, err := rs.ActiveService.GetVisit(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	existing.StudentID = req.StudentID
	existing.ActiveGroupID = req.ActiveGroupID
	existing.EntryTime = req.CheckInTime
	existing.ExitTime = req.CheckOutTime

	// Update visit
	if err := rs.ActiveService.UpdateVisit(r.Context(), existing); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid visit ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete visit
	if err := rs.ActiveService.DeleteVisit(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Visit deleted successfully")
}

// endVisit handles ending a visit
func (rs *Resource) endVisit(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid visit ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// End visit
	if err := rs.ActiveService.EndVisit(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("is_active", isActive)
	}

	// Get supervisors
	supervisors, err := rs.ActiveService.ListGroupSupervisors(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get supervisor
	supervisor, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Prepare response
	response := newSupervisorResponse(supervisor)

	common.Respond(w, r, http.StatusOK, response, "Supervisor retrieved successfully")
}

// getStaffSupervisions handles getting supervisions for a staff member
func (rs *Resource) getStaffSupervisions(w http.ResponseWriter, r *http.Request) {
	// Parse staff ID from URL
	staffID, err := strconv.ParseInt(chi.URLParam(r, "staffId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid staff ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get supervisions for staff
	supervisors, err := rs.ActiveService.FindSupervisorsByStaffID(r.Context(), staffID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	staffID, err := strconv.ParseInt(chi.URLParam(r, "staffId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid staff ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active supervisions for staff
	supervisors, err := rs.ActiveService.GetStaffActiveSupervisions(r.Context(), staffID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get supervisors for active group
	supervisors, err := rs.ActiveService.FindSupervisorsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &SupervisorRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing supervisor
	existing, err := rs.ActiveService.GetGroupSupervisor(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	existing.StaffID = req.StaffID
	existing.GroupID = req.ActiveGroupID
	existing.StartDate = req.StartTime
	existing.EndDate = req.EndTime

	// Update supervisor
	if err := rs.ActiveService.UpdateGroupSupervisor(r.Context(), existing); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete supervisor
	if err := rs.ActiveService.DeleteGroupSupervisor(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Supervisor deleted successfully")
}

// endSupervision handles ending a supervision
func (rs *Resource) endSupervision(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid supervisor ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// End supervision
	if err := rs.ActiveService.EndSupervision(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		isActive := activeStr == "true" || activeStr == "1"
		queryOptions.Filter.Equal("is_active", isActive)
	}

	// Get combined groups
	groups, err := rs.ActiveService.ListCombinedGroups(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid combined group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get combined group
	group, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Prepare response
	response := newCombinedGroupResponse(group)

	common.Respond(w, r, http.StatusOK, response, "Combined group retrieved successfully")
}

// getCombinedGroupGroups handles getting active groups in a combined group
func (rs *Resource) getCombinedGroupGroups(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid combined group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get combined group with groups
	combinedGroup, err := rs.ActiveService.GetCombinedGroupWithGroups(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create combined group
	group := &active.CombinedGroup{
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	// Create combined group
	if err := rs.ActiveService.CreateCombinedGroup(r.Context(), group); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Add groups to the combined group if provided
	if len(req.GroupIDs) > 0 {
		for _, groupID := range req.GroupIDs {
			if err := rs.ActiveService.AddGroupToCombination(r.Context(), group.ID, groupID); err != nil {
				// Log error but continue
				// TODO: Consider how to handle partial failures
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid combined group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &CombinedGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing combined group
	existing, err := rs.ActiveService.GetCombinedGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	existing.StartTime = req.StartTime
	existing.EndTime = req.EndTime

	// Update combined group
	if err := rs.ActiveService.UpdateCombinedGroup(r.Context(), existing); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid combined group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete combined group
	if err := rs.ActiveService.DeleteCombinedGroup(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Combined group deleted successfully")
}

// endCombinedGroup handles ending a combined group
func (rs *Resource) endCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid combined group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// End combined group
	if err := rs.ActiveService.EndCombinedGroup(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
	groupID, err := strconv.ParseInt(chi.URLParam(r, "groupId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get mappings for active group
	mappings, err := rs.ActiveService.GetGroupMappingsByActiveGroupID(r.Context(), groupID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	combinedID, err := strconv.ParseInt(chi.URLParam(r, "combinedId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid combined group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get mappings for combined group
	mappings, err := rs.ActiveService.GetGroupMappingsByCombinedGroupID(r.Context(), combinedID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Add group to combination
	if err := rs.ActiveService.AddGroupToCombination(r.Context(), req.CombinedGroupID, req.ActiveGroupID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get the mappings for verification
	mappings, err := rs.ActiveService.GetGroupMappingsByCombinedGroupID(r.Context(), req.CombinedGroupID)
	if err != nil {
		common.Respond(w, r, http.StatusOK, nil, "Group added to combination successfully")
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
		common.Respond(w, r, http.StatusOK, nil, "Group added to combination successfully")
		return
	}

	// Return the mapping
	response := newGroupMappingResponse(newMapping)
	common.Respond(w, r, http.StatusOK, response, "Group added to combination successfully")
}

// removeGroupFromCombination handles removing an active group from a combined group
func (rs *Resource) removeGroupFromCombination(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupMappingRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Remove group from combination
	if err := rs.ActiveService.RemoveGroupFromCombination(r.Context(), req.CombinedGroupID, req.ActiveGroupID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
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
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get total visits count
	totalVisitsCount, err := rs.ActiveService.GetTotalVisitsCount(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active visits count
	activeVisitsCount, err := rs.ActiveService.GetActiveVisitsCount(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	roomID, err := strconv.ParseInt(chi.URLParam(r, "roomId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get room utilization
	utilization, err := rs.ActiveService.GetRoomUtilization(r.Context(), roomID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
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
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid student ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get student attendance rate
	attendanceRate, err := rs.ActiveService.GetStudentAttendanceRate(r.Context(), studentID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	response := AnalyticsResponse{
		AttendanceRate: attendanceRate,
	}

	common.Respond(w, r, http.StatusOK, response, "Student attendance rate retrieved successfully")
}
