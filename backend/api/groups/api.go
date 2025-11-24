package groups

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the group API resource
type Resource struct {
	EducationService   educationSvc.Service
	ActiveService      activeService.Service
	UserService        userService.PersonService
	UserContextService userContextService.UserContextService
	StudentRepo        users.StudentRepository
	StaffRepo          users.StaffRepository
	SubstitutionRepo   education.GroupSubstitutionRepository
}

// NewResource creates a new groups resource
func NewResource(educationService educationSvc.Service, activeService activeService.Service, userService userService.PersonService, userContextService userContextService.UserContextService, studentRepo users.StudentRepository, substitutionRepo education.GroupSubstitutionRepository) *Resource {
	return &Resource{
		EducationService:   educationService,
		ActiveService:      activeService,
		UserService:        userService,
		UserContextService: userContextService,
		StudentRepo:        studentRepo,
		StaffRepo:          userService.StaffRepository(),
		SubstitutionRepo:   substitutionRepo,
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
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/students/room-status", rs.getGroupStudentsRoomStatus)
		r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/{id}/substitutions", rs.getGroupSubstitutions)

		// Write operations require groups:create, groups:update, or groups:delete permission
		r.With(authorize.RequiresPermission(permissions.GroupsCreate)).Post("/", rs.createGroup)
		r.With(authorize.RequiresPermission(permissions.GroupsUpdate)).Put("/{id}", rs.updateGroup)
		r.With(authorize.RequiresPermission(permissions.GroupsDelete)).Delete("/{id}", rs.deleteGroup)

		// Group transfer operations - authenticated users can transfer their own groups
		r.Route("/{id}/transfer", func(r chi.Router) {
			r.Post("/", rs.transferGroup)
			r.Delete("/", rs.cancelGroupTransfer)
			r.Delete("/{substitutionId}", rs.cancelSpecificTransfer)
		})
	})

	return r
}

// GroupResponse represents a group API response
type GroupResponse struct {
	ID               int64             `json:"id"`
	Name             string            `json:"name"`
	RoomID           *int64            `json:"room_id,omitempty"`
	Room             *Room             `json:"room,omitempty"`
	RepresentativeID *int64            `json:"representative_id,omitempty"`
	Representative   *TeacherResponse  `json:"representative,omitempty"`
	Teachers         []TeacherResponse `json:"teachers,omitempty"`
	StudentCount     int               `json:"student_count"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
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

// TransferGroupRequest represents a request to transfer group access to another user
type TransferGroupRequest struct {
	TargetUserID int64 `json:"target_user_id"`
}

// Bind validates the transfer group request
func (req *TransferGroupRequest) Bind(r *http.Request) error {
	if req.TargetUserID <= 0 {
		return errors.New("target_user_id is required")
	}
	return nil
}

// newGroupResponse converts a group model to a response object
func newGroupResponse(group *education.Group, teachers []*users.Teacher, studentCount int) GroupResponse {
	response := GroupResponse{
		ID:           group.ID,
		Name:         group.Name,
		RoomID:       group.RoomID,
		StudentCount: studentCount,
		CreatedAt:    group.CreatedAt,
		UpdatedAt:    group.UpdatedAt,
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

		// Get teachers for this group to show representative in list
		teachers, err := rs.EducationService.GetGroupTeachers(r.Context(), group.ID)
		if err != nil {
			// Log error but continue without teachers
			log.Printf("Failed to get teachers for group %d: %v", group.ID, err)
			teachers = []*users.Teacher{}
		}

		// Get student count for this group
		students, err := rs.StudentRepo.FindByGroupID(r.Context(), group.ID)
		studentCount := 0
		if err == nil {
			studentCount = len(students)
		}

		responses = append(responses, newGroupResponse(group, teachers, studentCount))
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

	// Get student count for this group
	students, err := rs.StudentRepo.FindByGroupID(r.Context(), id)
	studentCount := 0
	if err == nil {
		studentCount = len(students)
	}

	common.Respond(w, r, http.StatusOK, newGroupResponse(group, teachers, studentCount), "Group retrieved successfully")
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

	// Assign teachers to the group if any were provided
	if len(req.TeacherIDs) > 0 {
		if err := rs.EducationService.UpdateGroupTeachers(r.Context(), group.ID, req.TeacherIDs); err != nil {
			// Log the error but don't fail the entire operation
			log.Printf("Failed to assign teachers to group %d: %v", group.ID, err)
		}
	}

	// Get the created group with room details
	createdGroup, err := rs.EducationService.FindGroupWithRoom(r.Context(), group.ID)
	if err != nil {
		createdGroup = group // Fallback to the original group without room details
	}

	// Get teachers for the group
	teachers, _ := rs.EducationService.GetGroupTeachers(r.Context(), group.ID)

	// New group has no students yet
	studentCount := 0

	common.Respond(w, r, http.StatusCreated, newGroupResponse(createdGroup, teachers, studentCount), "Group created successfully")
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

	// Get student count for the updated group
	students, err := rs.StudentRepo.FindByGroupID(r.Context(), group.ID)
	studentCount := 0
	if err == nil {
		studentCount = len(students)
	}

	common.Respond(w, r, http.StatusOK, newGroupResponse(updatedGroup, teachers, studentCount), "Group updated successfully")
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
	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("group not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get user permissions to check authorization
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	// Check if user is supervisor of this group
	myGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		log.Printf("Error getting user groups: %v", err)
		myGroups = []*education.Group{}
	}

	// Determine if user can see full student details
	canAccessFullDetails := isAdmin
	if !canAccessFullDetails {
		for _, myGroup := range myGroups {
			if myGroup.ID == id {
				canAccessFullDetails = true
				break
			}
		}
	}

	// Get students for this group
	students, err := rs.StudentRepo.FindByGroupID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	locationSnapshot, snapshotErr := common.LoadStudentLocationSnapshot(r.Context(), rs.ActiveService, studentIDs)
	if snapshotErr != nil {
		log.Printf("Failed to batch load group student locations: %v", snapshotErr)
		locationSnapshot = nil
	}

	// Build response with person data for each student
	type StudentResponse struct {
		ID              int64  `json:"id"`
		PersonID        int64  `json:"person_id"`
		FirstName       string `json:"first_name"`
		LastName        string `json:"last_name"`
		SchoolClass     string `json:"school_class"`
		GroupID         int64  `json:"group_id"`
		GroupName       string `json:"group_name"`
		GuardianName    string `json:"guardian_name,omitempty"`
		GuardianContact string `json:"guardian_contact,omitempty"`
		GuardianEmail   string `json:"guardian_email,omitempty"`
		GuardianPhone   string `json:"guardian_phone,omitempty"`
		Location        string `json:"location,omitempty"`
		TagID           string `json:"tag_id,omitempty"`
	}

	responses := make([]StudentResponse, 0, len(students))
	for _, student := range students {
		// Get person data
		person, err := rs.UserService.Get(r.Context(), student.PersonID)
		if err != nil {
			// Skip this student if person not found
			log.Printf("Failed to get person data for student %d: %v", student.ID, err)
			continue
		}

		response := StudentResponse{
			ID:          student.ID,
			PersonID:    student.PersonID,
			FirstName:   person.FirstName,
			LastName:    person.LastName,
			SchoolClass: student.SchoolClass,
			GroupID:     id,
			GroupName:   group.Name,
		}

		// Include sensitive data only for authorized users
		if canAccessFullDetails {
			if student.GuardianName != nil {
				response.GuardianName = *student.GuardianName
			}
			if student.GuardianContact != nil {
				response.GuardianContact = *student.GuardianContact
			}
			if student.GuardianEmail != nil {
				response.GuardianEmail = *student.GuardianEmail
			}
			if student.GuardianPhone != nil {
				response.GuardianPhone = *student.GuardianPhone
			}
			if person.TagID != nil {
				response.TagID = *person.TagID
			}

		}

		// Limited data for non-supervisor staff
		if !canAccessFullDetails && student.GuardianName != nil {
			response.GuardianName = *student.GuardianName
		}

		// Location is derived from real-time attendance data
		if locationSnapshot != nil {
			response.Location = locationSnapshot.ResolveStudentLocation(student.ID, canAccessFullDetails)
		} else {
			response.Location = rs.resolveStudentLocation(r.Context(), student.ID, canAccessFullDetails)
		}

		responses = append(responses, response)
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Found %d students in group", len(responses)))
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

// getGroupStudentsRoomStatus handles getting room status for all students in a group
func (rs *Resource) getGroupStudentsRoomStatus(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid group ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get the educational group
	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("group not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check authorization - only group supervisors and admins can see this information
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	isAdmin := hasAdminPermissions(userPermissions)

	if !isAdmin {
		// Check if user supervises this educational group
		hasAccess := false
		educationGroups, err := rs.UserContextService.GetMyGroups(r.Context())
		if err == nil {
			for _, supervGroup := range educationGroups {
				if supervGroup.ID == id {
					hasAccess = true
					break
				}
			}
		}

		if !hasAccess {
			if err := render.Render(w, r, ErrorForbidden(errors.New("you do not supervise this group"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	}

	// Get all students in the group
	students, err := rs.StudentRepo.FindByGroupID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to get group students"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if the educational group has a room assigned
	if group.RoomID == nil {
		// No room assigned, all students are not in group room
		result := make(map[string]interface{})
		result["group_has_room"] = false
		result["student_room_status"] = make(map[string]interface{})

		for _, student := range students {
			studentStatus := map[string]interface{}{
				"in_group_room": false,
				"reason":        "group_no_room",
			}
			result["student_room_status"].(map[string]interface{})[strconv.FormatInt(student.ID, 10)] = studentStatus
		}

		common.Respond(w, r, http.StatusOK, result, "Group has no assigned room")
		return
	}

	// Group has a room, check each student's status
	result := make(map[string]interface{})
	result["group_has_room"] = true
	result["group_room_id"] = *group.RoomID
	studentStatuses := make(map[string]interface{})

	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	roomSnapshot, snapshotErr := common.LoadStudentLocationSnapshot(r.Context(), rs.ActiveService, studentIDs)
	if snapshotErr != nil {
		log.Printf("Failed to batch load student room locations: %v", snapshotErr)
		roomSnapshot = nil
	}

	for _, student := range students {
		studentStatus := map[string]interface{}{
			"in_group_room": false,
			"reason":        "no_active_visit",
		}

		var visit *active.Visit
		if roomSnapshot != nil {
			visit = roomSnapshot.Visits[student.ID]
		} else {
			if v, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID); err == nil {
				visit = v
			}
		}

		if visit != nil {
			var activeGroup *active.Group
			if roomSnapshot != nil {
				activeGroup = roomSnapshot.Groups[visit.ActiveGroupID]
			} else {
				if g, err := rs.ActiveService.GetActiveGroup(r.Context(), visit.ActiveGroupID); err == nil {
					activeGroup = g
				}
			}

			if activeGroup != nil {
				inGroupRoom := activeGroup.RoomID == *group.RoomID
				studentStatus["in_group_room"] = inGroupRoom
				studentStatus["current_room_id"] = activeGroup.RoomID

				if inGroupRoom {
					delete(studentStatus, "reason")
				} else {
					studentStatus["reason"] = "in_different_room"
				}
			}
		}

		// Get student's person data for display
		person, err := rs.UserService.Get(r.Context(), student.PersonID)
		if err == nil && person != nil {
			studentStatus["first_name"] = person.FirstName
			studentStatus["last_name"] = person.LastName
		}

		studentStatuses[strconv.FormatInt(student.ID, 10)] = studentStatus
	}

	result["student_room_status"] = studentStatuses
	common.Respond(w, r, http.StatusOK, result, "Student room status retrieved successfully")
}

// getGroupSubstitutions gets active substitutions for a specific group
func (rs *Resource) getGroupSubstitutions(w http.ResponseWriter, r *http.Request) {
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

	// Get active substitutions for this group
	date := time.Now()
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		parsedDate, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			date = parsedDate
		}
	}

	substitutions, err := rs.EducationService.GetActiveGroupSubstitutions(r.Context(), id, date)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, substitutions, "Group substitutions retrieved successfully")
}

// resolveStudentLocation determines the student's location string based on active attendance data.
func (rs *Resource) resolveStudentLocation(ctx context.Context, studentID int64, hasFullAccess bool) string {
	attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return "Abwesend"
	}

	if attendanceStatus.Status != "checked_in" {
		return "Abwesend"
	}

	if !hasFullAccess {
		return "Anwesend"
	}

	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil {
		return "Anwesend"
	}

	if currentVisit.ActiveGroupID <= 0 {
		return "Anwesend"
	}

	activeGroup, err := rs.ActiveService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return "Anwesend"
	}

	if activeGroup.Room != nil && activeGroup.Room.Name != "" {
		return fmt.Sprintf("Anwesend - %s", activeGroup.Room.Name)
	}

	return "Anwesend"
}

// Helper function to check if user has admin permissions
func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
}

// transferGroup handles POST /api/groups/{id}/transfer
// Allows a group leader to grant temporary access to another user until end of day
func (rs *Resource) transferGroup(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Ungültige Gruppen-ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request body
	req := &TransferGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		// Translate validation errors
		errMsg := "Ungültige Anfrage"
		if err.Error() == "target_user_id is required" {
			errMsg = "Bitte wähle einen Betreuer aus"
		}
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errMsg))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user's staff record
	currentStaff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du musst ein Mitarbeiter sein, um Gruppen zu übergeben"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user's teacher record (only teachers can lead groups)
	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Gruppen zu übergeben"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Verify that current user is actually a leader of this group
	myGroups, err := rs.EducationService.GetTeacherGroups(r.Context(), currentTeacher.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	isGroupLeader := false
	for _, group := range myGroups {
		if group.ID == groupID {
			isGroupLeader = true
			break
		}
	}

	if !isGroupLeader {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen vornehmen"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Verify target user exists and get their person record
	targetPerson, err := rs.UserService.Get(r.Context(), req.TargetUserID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("Der ausgewählte Betreuer wurde nicht gefunden"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get target user's staff record (must be staff to receive group access)
	targetStaff, err := rs.StaffRepo.FindByPersonID(r.Context(), targetPerson.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Der ausgewählte Betreuer ist kein Mitarbeiter"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Prevent self-transfer
	if targetStaff.ID == currentStaff.ID {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Du kannst die Gruppe nicht an dich selbst übergeben"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if this specific user already has access to this group (prevent duplicate transfers to same user)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	existingTransfers, err := rs.EducationService.GetActiveGroupSubstitutions(r.Context(), groupID, today)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Check if target user already has access to this group (prevent duplicate)
	for _, transfer := range existingTransfers {
		if transfer.RegularStaffID == nil && transfer.SubstituteStaffID == targetStaff.ID {
			// Load target name for better error message
			targetName := targetPerson.FirstName + " " + targetPerson.LastName
			errorMsg := fmt.Sprintf("Du hast diese Gruppe bereits an %s übergeben", targetName)
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New(errorMsg))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
	}

	// Calculate end of day (23:59:59 UTC)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)

	// Create substitution (without regular_staff_id = additional access, not replacement)
	substitution := &education.GroupSubstitution{
		GroupID:           groupID,
		RegularStaffID:    nil, // NULL = additional access, not replacement
		SubstituteStaffID: targetStaff.ID,
		StartDate:         today,
		EndDate:           endOfDay,
		Reason:            "Gruppenübergabe",
	}

	// Validate substitution data
	if err := substitution.Validate(); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create the transfer directly via repository (bypass service conflict check)
	// For group transfers, we WANT users to have multiple groups, so skip FindOverlapping check
	if err := rs.SubstitutionRepo.Create(r.Context(), substitution); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, map[string]interface{}{
		"substitution_id": substitution.ID,
		"group_id":        groupID,
		"target_staff_id": targetStaff.ID,
		"valid_until":     endOfDay.Format(time.RFC3339),
	}, "Group access transferred successfully")
}

// cancelGroupTransfer handles DELETE /api/groups/{id}/transfer
// Allows a group leader to cancel an active transfer they created
func (rs *Resource) cancelGroupTransfer(w http.ResponseWriter, r *http.Request) {
	// Parse group ID from URL
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Ungültige Gruppen-ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user's teacher record
	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Übertragungen zurückzunehmen"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Verify that current user is a leader of this group
	myGroups, err := rs.EducationService.GetTeacherGroups(r.Context(), currentTeacher.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	isGroupLeader := false
	for _, group := range myGroups {
		if group.ID == groupID {
			isGroupLeader = true
			break
		}
	}

	if !isGroupLeader {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen zurücknehmen"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Find active transfer for this group (only transfers, not admin substitutions)
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	activeTransfers, err := rs.EducationService.GetActiveGroupSubstitutions(r.Context(), groupID, today)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Find the transfer (regular_staff_id IS NULL)
	var transferToDelete *education.GroupSubstitution
	for _, transfer := range activeTransfers {
		if transfer.RegularStaffID == nil {
			transferToDelete = transfer
			break
		}
	}

	if transferToDelete == nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("Keine aktive Übertragung für diese Gruppe gefunden"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete the transfer
	if err := rs.EducationService.DeleteSubstitution(r.Context(), transferToDelete.ID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Group transfer cancelled successfully")
}

// cancelSpecificTransfer handles DELETE /api/groups/{id}/transfer/{substitutionId}
// Allows a group leader to cancel a specific transfer by substitution ID
func (rs *Resource) cancelSpecificTransfer(w http.ResponseWriter, r *http.Request) {
	// Parse group ID and substitution ID from URL
	groupID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Ungültige Gruppen-ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	substitutionID, err := strconv.ParseInt(chi.URLParam(r, "substitutionId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Ungültige Substitutions-ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current user's teacher record
	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Übertragungen zurückzunehmen"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Verify that current user is a leader of this group
	myGroups, err := rs.EducationService.GetTeacherGroups(r.Context(), currentTeacher.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	isGroupLeader := false
	for _, group := range myGroups {
		if group.ID == groupID {
			isGroupLeader = true
			break
		}
	}

	if !isGroupLeader {
		if err := render.Render(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen zurücknehmen"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Verify that the substitution exists and belongs to this group
	substitution, err := rs.SubstitutionRepo.FindByID(r.Context(), substitutionID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("Übertragung nicht gefunden"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Verify it's a transfer (not admin substitution) and belongs to this group
	if substitution.RegularStaffID != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Dies ist eine Admin-Vertretung und kann nicht hier gelöscht werden"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	if substitution.GroupID != groupID {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("Diese Übertragung gehört nicht zu dieser Gruppe"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete the specific transfer
	if err := rs.EducationService.DeleteSubstitution(r.Context(), substitutionID); err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Transfer cancelled successfully")
}
