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

// GroupStudentResponse represents a student in a group response
type GroupStudentResponse struct {
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

		// Group transfer operations - Self-service feature for group leaders
		//
		// DESIGN NOTE: No permission checks required (intentional design decision)
		// Authorization is based on group ownership, not permissions:
		// - User must be authenticated (JWT middleware from parent router)
		// - User must be a Teacher (checked in handler via GetCurrentTeacher)
		// - User must be assigned to this group (verified via education.group_teacher table)
		//
		// This differs from /api/substitutions (admin-only, multi-day coverage):
		// - Transfers: Any group leader, same-day only (expires 23:59 UTC), additional access
		// - Substitutions: Admin-only, configurable duration, managed via admin UI
		//
		// Both use the same database table (education.group_substitution), distinguished by:
		// - Transfers: regular_staff_id IS NULL (additional access)
		// - Substitutions: regular_staff_id IS NOT NULL (person replacement)
		r.Route("/{id}/transfer", func(r chi.Router) {
			r.Post("/", rs.transferGroup)
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
func (req *GroupRequest) Bind(_ *http.Request) error {
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
func (req *TransferGroupRequest) Bind(_ *http.Request) error {
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

// =============================================================================
// HELPER METHODS - Reduce code duplication for common parsing/validation
// =============================================================================

// parseAndGetGroup parses group ID from URL and returns the group if it exists.
// Returns nil and false if parsing fails or group doesn't exist (error already rendered).
func (rs *Resource) parseAndGetGroup(w http.ResponseWriter, r *http.Request) (*education.Group, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(common.MsgInvalidGroupID)))
		return nil, false
	}

	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return nil, false
	}

	return group, true
}

// getStudentCount returns the number of students in a group.
func (rs *Resource) getStudentCount(ctx context.Context, groupID int64) int {
	students, err := rs.StudentRepo.FindByGroupID(ctx, groupID)
	if err != nil {
		return 0
	}
	return len(students)
}

// isUserGroupLeader checks if the given teacher is a leader of the specified group.
// Returns true if the teacher leads the group, false otherwise.
func (rs *Resource) isUserGroupLeader(ctx context.Context, teacherID int64, groupID int64) (bool, error) {
	myGroups, err := rs.EducationService.GetTeacherGroups(ctx, teacherID)
	if err != nil {
		return false, err
	}

	for _, group := range myGroups {
		if group.ID == groupID {
			return true, nil
		}
	}
	return false, nil
}

// userHasGroupAccess checks if the current user has access to the specified group.
// Returns true if user is admin or supervises the group.
func (rs *Resource) userHasGroupAccess(r *http.Request, groupID int64) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if hasAdminPermissions(userPermissions) {
		return true
	}

	myGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		log.Printf("Error getting user groups: %v", err)
		return false
	}

	for _, myGroup := range myGroups {
		if myGroup.ID == groupID {
			return true
		}
	}
	return false
}

// populateGuardianDetails fills in guardian fields based on access permissions
func populateGuardianDetails(response *GroupStudentResponse, student *users.Student, person *users.Person, hasFullAccess bool) {
	// Full access users get all guardian details and tag ID
	if hasFullAccess {
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
		return
	}

	// Limited access: only guardian name visible
	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}
}

// buildStudentResponse creates a student response with all necessary data
func (rs *Resource) buildStudentResponse(
	ctx context.Context,
	student *users.Student,
	group *education.Group,
	hasFullAccess bool,
	locationSnapshot *common.StudentLocationSnapshot,
) *GroupStudentResponse {
	person, err := rs.UserService.Get(ctx, student.PersonID)
	if err != nil {
		log.Printf("Failed to get person data for student %d: %v", student.ID, err)
		return nil
	}

	response := &GroupStudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		FirstName:   person.FirstName,
		LastName:    person.LastName,
		SchoolClass: student.SchoolClass,
		GroupID:     group.ID,
		GroupName:   group.Name,
	}

	populateGuardianDetails(response, student, person, hasFullAccess)
	response.Location = rs.resolveLocationForStudent(ctx, student.ID, hasFullAccess, locationSnapshot)

	return response
}

// resolveLocationForStudent determines student location from snapshot or fallback
func (rs *Resource) resolveLocationForStudent(
	ctx context.Context,
	studentID int64,
	hasFullAccess bool,
	snapshot *common.StudentLocationSnapshot,
) string {
	if snapshot != nil {
		return snapshot.ResolveStudentLocation(studentID, hasFullAccess)
	}
	return rs.resolveStudentLocation(ctx, studentID, hasFullAccess)
}

// getStudentVisit retrieves a student's current visit from snapshot or service
func (rs *Resource) getStudentVisit(ctx context.Context, studentID int64, snapshot *common.StudentLocationSnapshot) *active.Visit {
	if snapshot != nil {
		return snapshot.Visits[studentID]
	}
	visit, err := rs.ActiveService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		return nil
	}
	return visit
}

// getVisitActiveGroup retrieves the active group for a visit from snapshot or service
func (rs *Resource) getVisitActiveGroup(ctx context.Context, visit *active.Visit, snapshot *common.StudentLocationSnapshot) *active.Group {
	if snapshot != nil {
		return snapshot.Groups[visit.ActiveGroupID]
	}
	group, err := rs.ActiveService.GetActiveGroup(ctx, visit.ActiveGroupID)
	if err != nil {
		return nil
	}
	return group
}

// buildStudentRoomStatus creates the room status map for a single student
func (rs *Resource) buildStudentRoomStatus(
	ctx context.Context,
	student *users.Student,
	groupRoomID int64,
	snapshot *common.StudentLocationSnapshot,
) map[string]interface{} {
	status := map[string]interface{}{
		"in_group_room": false,
		"reason":        "no_active_visit",
	}

	visit := rs.getStudentVisit(ctx, student.ID, snapshot)
	if visit == nil {
		rs.addPersonDataToStatus(ctx, status, student.PersonID)
		return status
	}

	activeGroup := rs.getVisitActiveGroup(ctx, visit, snapshot)
	if activeGroup == nil {
		rs.addPersonDataToStatus(ctx, status, student.PersonID)
		return status
	}

	inGroupRoom := activeGroup.RoomID == groupRoomID
	status["in_group_room"] = inGroupRoom
	status["current_room_id"] = activeGroup.RoomID

	if inGroupRoom {
		delete(status, "reason")
	} else {
		status["reason"] = "in_different_room"
	}

	rs.addPersonDataToStatus(ctx, status, student.PersonID)
	return status
}

// addPersonDataToStatus adds first_name and last_name to status map
func (rs *Resource) addPersonDataToStatus(ctx context.Context, status map[string]interface{}, personID int64) {
	person, err := rs.UserService.Get(ctx, personID)
	if err == nil && person != nil {
		status["first_name"] = person.FirstName
		status["last_name"] = person.LastName
	}
}

// buildNoRoomResponse creates the response when group has no room assigned
func buildNoRoomResponse(students []*users.Student) map[string]interface{} {
	result := map[string]interface{}{
		"group_has_room":      false,
		"student_room_status": make(map[string]interface{}),
	}

	statusMap := result["student_room_status"].(map[string]interface{})
	for _, student := range students {
		statusMap[strconv.FormatInt(student.ID, 10)] = map[string]interface{}{
			"in_group_room": false,
			"reason":        "group_no_room",
		}
	}
	return result
}

// =============================================================================
// GROUP HANDLERS
// =============================================================================

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
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)
	queryOptions.Filter = filter

	// Get all groups
	groups, err := rs.EducationService.ListGroups(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
		studentCount := rs.getStudentCount(r.Context(), group.ID)

		responses = append(responses, newGroupResponse(group, teachers, studentCount))
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(responses)}, "Groups retrieved successfully")
}

// getGroup handles getting a group by ID
func (rs *Resource) getGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidGroupID)
	if !ok {
		return
	}

	// Get group with room details
	group, err := rs.EducationService.FindGroupWithRoom(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgGroupNotFound)))
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
	studentCount := rs.getStudentCount(r.Context(), id)

	common.Respond(w, r, http.StatusOK, newGroupResponse(group, teachers, studentCount), "Group retrieved successfully")
}

// createGroup handles creating a new group
func (rs *Resource) createGroup(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &GroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create group
	group := &education.Group{
		Name:   req.Name,
		RoomID: req.RoomID,
	}

	if err := rs.EducationService.CreateGroup(r.Context(), group); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
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
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidGroupID)
	if !ok {
		return
	}

	// Parse request
	req := &GroupRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Get existing group
	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return
	}

	// Update fields
	group.Name = req.Name
	group.RoomID = req.RoomID

	// Update group
	if err := rs.EducationService.UpdateGroup(r.Context(), group); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
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
	studentCount := rs.getStudentCount(r.Context(), group.ID)

	common.Respond(w, r, http.StatusOK, newGroupResponse(updatedGroup, teachers, studentCount), "Group updated successfully")
}

// deleteGroup handles deleting a group
func (rs *Resource) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidGroupID)
	if !ok {
		return
	}

	// Delete group
	if err := rs.EducationService.DeleteGroup(r.Context(), id); err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Group deleted successfully")
}

// getGroupStudents gets all students in a specific group
func (rs *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	// Parse and get group
	group, ok := rs.parseAndGetGroup(w, r)
	if !ok {
		return
	}
	id := group.ID

	// Determine if user can see full student details (admin or group supervisor)
	canAccessFullDetails := rs.userHasGroupAccess(r, id)

	// Get students for this group
	students, err := rs.StudentRepo.FindByGroupID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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
	responses := make([]GroupStudentResponse, 0, len(students))
	for _, student := range students {
		response := rs.buildStudentResponse(r.Context(), student, group, canAccessFullDetails, locationSnapshot)
		if response != nil {
			responses = append(responses, *response)
		}
	}

	common.Respond(w, r, http.StatusOK, responses, fmt.Sprintf("Found %d students in group", len(responses)))
}

// getGroupSupervisors gets all supervisors (teachers) for a specific group
func (rs *Resource) getGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	// Parse and get group
	group, ok := rs.parseAndGetGroup(w, r)
	if !ok {
		return
	}

	// Get teachers/supervisors for this group
	teachers, err := rs.EducationService.GetGroupTeachers(r.Context(), group.ID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
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
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidGroupID)
	if !ok {
		return
	}

	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return
	}

	if !rs.userHasGroupAccess(r, id) {
		common.RenderError(w, r, ErrorForbidden(errors.New("you do not supervise this group")))
		return
	}

	students, err := rs.StudentRepo.FindByGroupID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("failed to get group students")))
		return
	}

	// Handle case where group has no room assigned
	if group.RoomID == nil {
		common.Respond(w, r, http.StatusOK, buildNoRoomResponse(students), "Group has no assigned room")
		return
	}

	// Build room status for each student
	result := rs.buildRoomStatusResponse(r.Context(), students, *group.RoomID)
	common.Respond(w, r, http.StatusOK, result, "Student room status retrieved successfully")
}

// buildRoomStatusResponse creates the full response for room status with student details
func (rs *Resource) buildRoomStatusResponse(ctx context.Context, students []*users.Student, groupRoomID int64) map[string]interface{} {
	result := map[string]interface{}{
		"group_has_room": true,
		"group_room_id":  groupRoomID,
	}

	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}

	snapshot, snapshotErr := common.LoadStudentLocationSnapshot(ctx, rs.ActiveService, studentIDs)
	if snapshotErr != nil {
		log.Printf("Failed to batch load student room locations: %v", snapshotErr)
		snapshot = nil
	}

	studentStatuses := make(map[string]interface{})
	for _, student := range students {
		studentStatuses[strconv.FormatInt(student.ID, 10)] = rs.buildStudentRoomStatus(ctx, student, groupRoomID, snapshot)
	}

	result["student_room_status"] = studentStatuses
	return result
}

// getGroupSubstitutions gets active substitutions for a specific group
func (rs *Resource) getGroupSubstitutions(w http.ResponseWriter, r *http.Request) {
	// Parse and get group
	group, ok := rs.parseAndGetGroup(w, r)
	if !ok {
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

	substitutions, err := rs.EducationService.GetActiveGroupSubstitutions(r.Context(), group.ID, date)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
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

// validateGroupLeaderAccess ensures current user is a teacher who leads the specified group
func (rs *Resource) validateGroupLeaderAccess(w http.ResponseWriter, r *http.Request, groupID int64) (*users.Staff, *users.Teacher, bool) {
	currentStaff, err := rs.UserContextService.GetCurrentStaff(r.Context())
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du musst ein Mitarbeiter sein, um Gruppen zu übergeben")))
		return nil, nil, false
	}

	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Gruppen zu übergeben")))
		return nil, nil, false
	}

	isGroupLeader, err := rs.isUserGroupLeader(r.Context(), currentTeacher.ID, groupID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return nil, nil, false
	}

	if !isGroupLeader {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen vornehmen")))
		return nil, nil, false
	}

	return currentStaff, currentTeacher, true
}

// resolveTargetStaff validates and retrieves the target staff for a group transfer
func (rs *Resource) resolveTargetStaff(w http.ResponseWriter, r *http.Request, targetUserID int64) (*users.Person, *users.Staff, bool) {
	targetPerson, err := rs.UserService.Get(r.Context(), targetUserID)
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorNotFound(errors.New("Der ausgewählte Betreuer wurde nicht gefunden")))
		return nil, nil, false
	}

	targetStaff, err := rs.StaffRepo.FindByPersonID(r.Context(), targetPerson.ID)
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Der ausgewählte Betreuer ist kein Mitarbeiter")))
		return nil, nil, false
	}

	return targetPerson, targetStaff, true
}

// translateTransferRequestError converts transfer request errors to German messages
func (rs *Resource) translateTransferRequestError(err error) string {
	if err.Error() == "target_user_id is required" {
		return "Bitte wähle einen Betreuer aus"
	}
	return "Ungültige Anfrage"
}

// checkDuplicateTransfer verifies target doesn't already have access to this group
func (rs *Resource) checkDuplicateTransfer(w http.ResponseWriter, r *http.Request, groupID int64, targetStaffID int64, targetPerson *users.Person) bool {
	today := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC)
	existingTransfers, err := rs.EducationService.GetActiveGroupSubstitutions(r.Context(), groupID, today)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return false
	}

	for _, transfer := range existingTransfers {
		if transfer.RegularStaffID == nil && transfer.SubstituteStaffID == targetStaffID {
			targetName := targetPerson.FirstName + " " + targetPerson.LastName
			errorMsg := fmt.Sprintf("Du hast diese Gruppe bereits an %s übergeben", targetName)
			common.RenderError(w, r, ErrorInvalidRequest(errors.New(errorMsg)))
			return false
		}
	}
	return true
}

// transferGroup handles POST /api/groups/{id}/transfer
// Allows a group leader to grant temporary access to another user until end of day
func (rs *Resource) transferGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := common.ParseInt64IDWithError(w, r, "id", "Ungültige Gruppen-ID")
	if !ok {
		return
	}

	req := &TransferGroupRequest{}
	if err := render.Bind(r, req); err != nil {
		errMsg := rs.translateTransferRequestError(err)
		common.RenderError(w, r, ErrorInvalidRequest(errors.New(errMsg)))
		return
	}

	currentStaff, _, ok := rs.validateGroupLeaderAccess(w, r, groupID)
	if !ok {
		return
	}

	targetPerson, targetStaff, ok := rs.resolveTargetStaff(w, r, req.TargetUserID)
	if !ok {
		return
	}

	if targetStaff.ID == currentStaff.ID {
		//nolint:staticcheck // ST1005: German user-facing message
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Du kannst die Gruppe nicht an dich selbst übergeben")))
		return
	}

	if !rs.checkDuplicateTransfer(w, r, groupID, targetStaff.ID, targetPerson) {
		return
	}

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
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
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Create the transfer directly via repository (bypass service conflict check)
	// For group transfers, we WANT users to have multiple groups, so skip FindOverlapping check
	if err := rs.SubstitutionRepo.Create(r.Context(), substitution); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, map[string]interface{}{
		"substitution_id": substitution.ID,
		"group_id":        groupID,
		"target_staff_id": targetStaff.ID,
		"valid_until":     endOfDay.Format(time.RFC3339),
	}, "Group access transferred successfully")
}

// cancelSpecificTransfer handles DELETE /api/groups/{id}/transfer/{substitutionId}
// Allows a group leader to cancel a specific transfer by substitution ID
func (rs *Resource) cancelSpecificTransfer(w http.ResponseWriter, r *http.Request) {
	groupID, ok := common.ParseInt64IDWithError(w, r, "id", "Ungültige Gruppen-ID")
	if !ok {
		return
	}

	substitutionID, ok := common.ParseInt64IDWithError(w, r, "substitutionId", "Ungültige Substitutions-ID")
	if !ok {
		return
	}

	// Get current user's teacher record
	currentTeacher, err := rs.UserContextService.GetCurrentTeacher(r.Context())
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message, capitalization is correct
		common.RenderError(w, r, ErrorForbidden(errors.New("Du musst ein Gruppenleiter sein, um Übertragungen zurückzunehmen")))
		return
	}

	// Verify that current user is a leader of this group
	isGroupLeader, err := rs.isUserGroupLeader(r.Context(), currentTeacher.ID, groupID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	if !isGroupLeader {
		//nolint:staticcheck // ST1005: German user-facing message, capitalization is correct
		common.RenderError(w, r, ErrorForbidden(errors.New("Du bist kein Leiter dieser Gruppe. Nur der Original-Gruppenleiter kann Übertragungen zurücknehmen")))
		return
	}

	// Verify that the substitution exists and belongs to this group
	substitution, err := rs.SubstitutionRepo.FindByID(r.Context(), substitutionID)
	if err != nil {
		//nolint:staticcheck // ST1005: German user-facing message, capitalization is correct
		common.RenderError(w, r, ErrorNotFound(errors.New("Übertragung nicht gefunden")))
		return
	}

	// Verify it's a transfer (not admin substitution) and belongs to this group
	if substitution.RegularStaffID != nil {
		//nolint:staticcheck // ST1005: German user-facing message, capitalization is correct
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Dies ist eine Admin-Vertretung und kann nicht hier gelöscht werden")))
		return
	}

	if substitution.GroupID != groupID {
		//nolint:staticcheck // ST1005: German user-facing message, capitalization is correct
		common.RenderError(w, r, ErrorInvalidRequest(errors.New("Diese Übertragung gehört nicht zu dieser Gruppe")))
		return
	}

	// Delete the specific transfer
	if err := rs.EducationService.DeleteSubstitution(r.Context(), substitutionID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Transfer cancelled successfully")
}

// =============================================================================
// EXPORTED HANDLER METHODS FOR TESTING
// =============================================================================

// ListGroupsHandler returns the listGroups handler for testing
func (rs *Resource) ListGroupsHandler() http.HandlerFunc {
	return rs.listGroups
}

// GetGroupHandler returns the getGroup handler for testing
func (rs *Resource) GetGroupHandler() http.HandlerFunc {
	return rs.getGroup
}

// CreateGroupHandler returns the createGroup handler for testing
func (rs *Resource) CreateGroupHandler() http.HandlerFunc {
	return rs.createGroup
}

// UpdateGroupHandler returns the updateGroup handler for testing
func (rs *Resource) UpdateGroupHandler() http.HandlerFunc {
	return rs.updateGroup
}

// DeleteGroupHandler returns the deleteGroup handler for testing
func (rs *Resource) DeleteGroupHandler() http.HandlerFunc {
	return rs.deleteGroup
}

// GetGroupStudentsHandler returns the getGroupStudents handler for testing
func (rs *Resource) GetGroupStudentsHandler() http.HandlerFunc {
	return rs.getGroupStudents
}

// GetGroupSupervisorsHandler returns the getGroupSupervisors handler for testing
func (rs *Resource) GetGroupSupervisorsHandler() http.HandlerFunc {
	return rs.getGroupSupervisors
}

// GetGroupStudentsRoomStatusHandler returns the getGroupStudentsRoomStatus handler for testing
func (rs *Resource) GetGroupStudentsRoomStatusHandler() http.HandlerFunc {
	return rs.getGroupStudentsRoomStatus
}

// GetGroupSubstitutionsHandler returns the getGroupSubstitutions handler for testing
func (rs *Resource) GetGroupSubstitutionsHandler() http.HandlerFunc {
	return rs.getGroupSubstitutions
}

// TransferGroupHandler returns the transferGroup handler for testing
func (rs *Resource) TransferGroupHandler() http.HandlerFunc {
	return rs.transferGroup
}

// CancelSpecificTransferHandler returns the cancelSpecificTransfer handler for testing
func (rs *Resource) CancelSpecificTransferHandler() http.HandlerFunc {
	return rs.cancelSpecificTransfer
}
