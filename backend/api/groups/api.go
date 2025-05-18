package groups

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
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
)

// Resource defines the group API resource
type Resource struct {
	EducationService educationSvc.Service
}

// NewResource creates a new groups resource
func NewResource(educationService educationSvc.Service) *Resource {
	return &Resource{
		EducationService: educationService,
	}
}

// Router returns a configured router for group endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Read operations only require groups:read permission
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listGroups)
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}", rs.getGroup)
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/students", rs.getGroupStudents)
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/supervisors", rs.getGroupSupervisors)

		// Write operations require groups:create, groups:update, or groups:delete permission
		r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createGroup)
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateGroup)
		r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteGroup)
	})

	return r
}

// GroupResponse represents a group API response
type GroupResponse struct {
	ID                 int64             `json:"id"`
	Name               string            `json:"name"`
	RoomID             *int64            `json:"room_id,omitempty"`
	Room               *Room             `json:"room,omitempty"`
	RepresentativeID   *int64            `json:"representative_id,omitempty"`
	Representative     *TeacherResponse  `json:"representative,omitempty"`
	Teachers           []TeacherResponse `json:"teachers,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// TeacherResponse represents a teacher in API responses
type TeacherResponse struct {
	ID             int64  `json:"id"`
	StaffID        int64  `json:"staff_id"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Specialization string `json:"specialization"`
	Role           string `json:"role,omitempty"`
	FullName       string `json:"full_name"`
}

// Room represents a simplified room for inclusion in group responses
type Room struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// GroupRequest represents a group creation/update request
type GroupRequest struct {
	Name       string  `json:"name"`
	RoomID     *int64  `json:"room_id,omitempty"`
	TeacherIDs []int64 `json:"teacher_ids,omitempty"`
}

// Bind validates the group request
func (req *GroupRequest) Bind(r *http.Request) error {
	if req.Name == "" {
		return errors.New("group name is required")
	}
	return nil
}

// newGroupResponse converts a group model to a response object
func newGroupResponse(group *education.Group, teachers []*users.Teacher) GroupResponse {
	response := GroupResponse{
		ID:        group.ID,
		Name:      group.Name,
		RoomID:    group.RoomID,
		CreatedAt: group.CreatedAt,
		UpdatedAt: group.UpdatedAt,
	}

	// Add room details if available
	if group.Room != nil {
		response.Room = &Room{
			ID:   group.Room.ID,
			Name: group.Room.Name,
		}
	}

	// Add teacher details if available
	if len(teachers) > 0 {
		teacherResponses := make([]TeacherResponse, 0, len(teachers))
		
		// First teacher is the representative by convention
		firstTeacher := teachers[0]
		response.RepresentativeID = &firstTeacher.ID
		
		// Convert all teachers to response format
		for _, teacher := range teachers {
			teacherResp := TeacherResponse{
				ID:             teacher.ID,
				StaffID:        teacher.StaffID,
				Specialization: teacher.Specialization,
				Role:           teacher.Role,
				FullName:       teacher.GetFullName(),
			}
			
			// Extract first and last name from staff if available
			if teacher.Staff != nil && teacher.Staff.Person != nil {
				teacherResp.FirstName = teacher.Staff.Person.FirstName
				teacherResp.LastName = teacher.Staff.Person.LastName
			}
			
			teacherResponses = append(teacherResponses, teacherResp)
		}
		
		// Set first teacher as representative
		response.Representative = &teacherResponses[0]
		response.Teachers = teacherResponses
	}

	return response
}

// listGroups handles listing all groups with optional filtering
func (rs *Resource) listGroups(w http.ResponseWriter, r *http.Request) {
	// Create query options
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	name := r.URL.Query().Get("name")
	roomIDStr := r.URL.Query().Get("room_id")

	// Create filter
	filter := base.NewFilter()

	// Apply filters
	if name != "" {
		filter.ILike("name", "%"+name+"%")
	}

	if roomIDStr != "" {
		roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
		if err == nil {
			filter.Equal("room_id", roomID)
		}
	}

	// Add pagination
	page := 1
	pageSize := 50

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 {
			pageSize = ps
		}
	}

	queryOptions.WithPagination(page, pageSize)
	queryOptions.Filter = filter

	// Get all groups
	groups, err := rs.EducationService.ListGroups(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]GroupResponse, 0, len(groups))
	for _, group := range groups {
		// If group has room ID but room isn't loaded, fetch the room details
		if group.HasRoom() && group.Room == nil {
			groupWithRoom, err := rs.EducationService.FindGroupWithRoom(r.Context(), group.ID)
			if err == nil {
				group = groupWithRoom
			}
		}
		// For list operations, don't include teachers for performance
		responses = append(responses, newGroupResponse(group, nil))
	}

	common.RespondWithPagination(w, r, http.StatusOK, responses, page, pageSize, len(responses), "Groups retrieved successfully")
}

// getGroup handles getting a group by ID
func (rs *Resource) getGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get group with room details
	group, err := rs.EducationService.FindGroupWithRoom(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("group not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get teachers for this group
	teachers, err := rs.EducationService.GetGroupTeachers(r.Context(), id)
	if err != nil {
		// Log error but continue without teachers
		log.Printf("Failed to get teachers for group %d: %v", id, err)
		teachers = []*users.Teacher{}
	}

	common.Respond(w, r, http.StatusOK, newGroupResponse(group, teachers), "Group retrieved successfully")
}

// createGroup handles creating a new group
func (rs *Resource) createGroup(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create group
	group := &education.Group{
		Name:   req.Name,
		RoomID: req.RoomID,
	}

	if err := rs.EducationService.CreateGroup(r.Context(), group); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get the created group with room details
	createdGroup, err := rs.EducationService.FindGroupWithRoom(r.Context(), group.ID)
	if err != nil {
		createdGroup = group // Fallback to the original group without room details
	}

	// Get teachers for the group (should be empty for new groups)
	teachers, _ := rs.EducationService.GetGroupTeachers(r.Context(), group.ID)

	common.Respond(w, r, http.StatusCreated, newGroupResponse(createdGroup, teachers), "Group created successfully")
}

// updateGroup handles updating a group
func (rs *Resource) updateGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &GroupRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing group
	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("group not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	group.Name = req.Name
	group.RoomID = req.RoomID

	// Update group
	if err := rs.EducationService.UpdateGroup(r.Context(), group); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Update teacher assignments if provided
	if req.TeacherIDs != nil {
		if err := rs.EducationService.UpdateGroupTeachers(r.Context(), group.ID, req.TeacherIDs); err != nil {
			log.Printf("Error updating group teachers: %v", err)
			// Continue anyway - the group update was successful
		}
	}

	// Get updated group with room details
	updatedGroup, err := rs.EducationService.FindGroupWithRoom(r.Context(), group.ID)
	if err != nil {
		updatedGroup = group // Fallback to the original updated group without room details
	}

	// Get teachers for the updated group
	teachers, _ := rs.EducationService.GetGroupTeachers(r.Context(), group.ID)

	common.Respond(w, r, http.StatusOK, newGroupResponse(updatedGroup, teachers), "Group updated successfully")
}

// deleteGroup handles deleting a group
func (rs *Resource) deleteGroup(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete group
	if err := rs.EducationService.DeleteGroup(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Group deleted successfully")
}

// getGroupStudents gets all students in a specific group
func (rs *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if group exists
	_, err = rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("group not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// TODO: Get students for this group
	// For this implementation, we'll return an empty array as the student
	// repository needs to be integrated. In a real implementation, we would
	// fetch students from the student repository with GroupID filter.
	common.Respond(w, r, http.StatusOK, []interface{}{}, "No students in this group yet")
}

// getGroupSupervisors gets all supervisors (teachers) for a specific group
func (rs *Resource) getGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if group exists
	_, err = rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("group not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get teachers/supervisors for this group
	teachers, err := rs.EducationService.GetGroupTeachers(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Map to response objects
	type TeacherResponse struct {
		ID             int64     `json:"id"`
		StaffID        int64     `json:"staff_id"`
		Specialization string    `json:"specialization"`
		Role           string    `json:"role,omitempty"`
		Qualifications string    `json:"qualifications,omitempty"`
		FullName       string    `json:"full_name"`
		CreatedAt      time.Time `json:"created_at"`
		UpdatedAt      time.Time `json:"updated_at"`
	}

	responses := make([]TeacherResponse, 0, len(teachers))
	for _, teacher := range teachers {
		responses = append(responses, TeacherResponse{
			ID:             teacher.ID,
			StaffID:        teacher.StaffID,
			Specialization: teacher.Specialization,
			Role:           teacher.Role,
			Qualifications: teacher.Qualifications,
			FullName:       teacher.GetFullName(),
			CreatedAt:      teacher.CreatedAt,
			UpdatedAt:      teacher.UpdatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Group supervisors retrieved successfully")
}
