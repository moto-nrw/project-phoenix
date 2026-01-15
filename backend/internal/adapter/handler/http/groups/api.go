package groups

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	activeService "github.com/moto-nrw/project-phoenix/internal/core/service/active"
	educationSvc "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	userContextService "github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
	userService "github.com/moto-nrw/project-phoenix/internal/core/service/users"
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
	StudentService     userService.StudentService
}

// NewResource creates a new groups resource
func NewResource(educationService educationSvc.Service, activeService activeService.Service, userService userService.PersonService, userContextService userContextService.UserContextService, studentService userService.StudentService) *Resource {
	return &Resource{
		EducationService:   educationService,
		ActiveService:      activeService,
		UserService:        userService,
		UserContextService: userContextService,
		StudentService:     studentService,
	}
}

// Router returns a configured router for group endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

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
	students, err := rs.StudentService.FindByGroupID(ctx, groupID)
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

// hasAdminPermissions checks if user has admin permissions
func hasAdminPermissions(permissions []string) bool {
	for _, perm := range permissions {
		if perm == "admin:*" || perm == "*:*" {
			return true
		}
	}
	return false
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
		logger.Logger.WithError(err).Error("Error getting user groups")
		return false
	}

	for _, myGroup := range myGroups {
		if myGroup.ID == groupID {
			return true
		}
	}
	return false
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
	students, err := rs.StudentService.FindByGroupID(r.Context(), id)
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
		logger.Logger.WithError(snapshotErr).Warn("Failed to batch load group student locations")
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

	students, err := rs.StudentService.FindByGroupID(r.Context(), id)
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
