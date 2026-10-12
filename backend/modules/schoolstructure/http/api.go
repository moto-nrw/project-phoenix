// Package schoolstructurehttp is the School Structure HTTP adapter of the group routes
// under /api/groups (#2742). It drives the owner's group service and reads
// children and persons through the People Directory port the root binds.
package schoolstructurehttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CallerGroups resolves the educational groups of the request's caller.
type CallerGroups interface {
	GetMyGroups(ctx context.Context) ([]*domain.Group, error)
}

// GroupPeople is the People Directory side of the group routes: the children
// of a group and the persons behind them. The root binds it to the retained
// person service.
type GroupPeople interface {
	// GroupStudents returns the children of a group.
	GroupStudents(ctx context.Context, groupID int64) ([]peopledirectory.StudentRecord, error)
	// CountStudentsByGroupIDs counts the children of each group.
	CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)
	// FindPerson returns one person.
	FindPerson(ctx context.Context, personID int64) (peopledirectory.Person, error)
	// ListPersonsByID returns the persons keyed by ID; an unknown ID is absent.
	ListPersonsByID(ctx context.Context, personIDs []int64) (map[int64]peopledirectory.Person, error)
}

// Resource defines the group API resource
type Resource struct {
	EducationService   GroupService
	ActiveService      studentpresence.Presence
	People             GroupPeople
	UserContextService CallerGroups
}

// NewResource creates a new groups resource
func NewResource(educationService GroupService, activeService studentpresence.Presence, people GroupPeople, userContextService CallerGroups) *Resource {
	return &Resource{
		EducationService:   educationService,
		ActiveService:      activeService,
		People:             people,
		UserContextService: userContextService,
	}
}

// Router returns a configured router for group endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {

		// Read operations only require groups:read permission
		r.With(common.RequiresPermission(permissions.GroupsRead), withTx).Get("/", rs.listGroups)
		r.With(common.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}", rs.getGroup)
		r.With(common.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/students", rs.getGroupStudents)
		r.With(common.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/supervisors", rs.getGroupSupervisors)
		r.With(common.RequiresPermission(permissions.GroupsRead), withTx).Get("/{id}/students/room-status", rs.getGroupStudentsRoomStatus)

		// Write operations require groups:create, groups:update, or groups:delete permission
		r.With(common.RequiresPermission(permissions.GroupsCreate), withTx).Post("/", rs.createGroup)
		r.With(common.RequiresPermission(permissions.GroupsUpdate), withTx).Put("/{id}", rs.updateGroup)
		r.With(common.RequiresPermission(permissions.GroupsDelete), withTx).Delete("/{id}", rs.deleteGroup)
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
func newGroupResponse(group *education.Group, teachers []*education.Teacher, studentCount int) GroupResponse {
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
				FullName:       teacher.FullName(),
			}

			// Extract first and last name from staff if available
			if teacher.Person != nil {
				teacherResp.FirstName = teacher.Person.FirstName
				teacherResp.LastName = teacher.Person.LastName
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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidGroupID)))
		return nil, false
	}

	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return nil, false
	}

	return group, true
}

// userHasGroupAccess checks if the current user has access to the specified group.
// Returns true if user is admin or supervises the group.
func (rs *Resource) userHasGroupAccess(r *http.Request, groupID int64) bool {
	userPermissions := jwt.PermissionsFromCtx(r.Context())
	if securityruntime.HasAdminWildcard(userPermissions) {
		return true
	}

	myGroups, err := rs.UserContextService.GetMyGroups(r.Context())
	if err != nil {
		slog.Default().Error("failed to get user groups", slog.String("error", err.Error()))
		return false
	}

	for _, myGroup := range myGroups {
		if myGroup.ID == groupID {
			return true
		}
	}
	return false
}

// =============================================================================
// GROUP HANDLERS
// =============================================================================

// listGroups handles listing all groups with optional filtering
func (rs *Resource) listGroups(w http.ResponseWriter, r *http.Request) {
	// Add filters if provided
	name := r.URL.Query().Get("name")
	roomIDStr := r.URL.Query().Get("room_id")
	query := &education.GroupListQuery{NameContains: name}

	if roomIDStr != "" {
		roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
		if err == nil {
			query.RoomID = &roomID
		}
	}

	page, pageSize := common.ParsePagination(r)
	query.Limit = pageSize
	query.Offset = (page - 1) * pageSize

	// Get all groups
	groups, err := rs.EducationService.ListGroups(r.Context(), query)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Collect group IDs for batch operations
	groupIDs := make([]int64, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}

	// Batch load student counts (1 query instead of N)
	studentCounts, err := rs.People.CountStudentsByGroupIDs(r.Context(), groupIDs)
	if err != nil {
		slog.Default().Warn("failed to batch load student counts", slog.String("error", err.Error()))
		studentCounts = make(map[int64]int)
	}

	// Batch load teachers for all groups (2 queries instead of ~N*4)
	teachersByGroup, err := rs.EducationService.GetTeachersForGroups(r.Context(), groupIDs)
	if err != nil {
		slog.Default().Warn("failed to batch load teachers", slog.String("error", err.Error()))
		teachersByGroup = make(map[int64][]*education.Teacher)
	}

	// Build response using pre-loaded data
	responses := make([]GroupResponse, 0, len(groups))
	for _, group := range groups {
		teachers := teachersByGroup[group.ID]
		studentCount := studentCounts[group.ID]
		responses = append(responses, newGroupResponse(group, teachers, studentCount))
	}

	// Count total for pagination (same filters, no LIMIT/OFFSET)
	total, countErr := rs.EducationService.CountGroups(r.Context(), query)
	if countErr != nil {
		total = len(responses)
	}

	common.RespondPaginated(w, r, http.StatusOK, responses, common.PaginationParams{Page: page, PageSize: pageSize, Total: total}, "Groups retrieved successfully")
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
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return
	}

	// Get teachers for this group
	teachers, err := rs.EducationService.GetGroupTeachers(r.Context(), id)
	if err != nil {
		// Log error but continue without teachers
		slog.Default().Warn("failed to get teachers for group",
			slog.Int64("group_id", id),
			slog.String("error", err.Error()))
		teachers = []*education.Teacher{}
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Create group
	group := &education.Group{
		Name:   req.Name,
		RoomID: req.RoomID,
	}

	if err := tenant.WithinCurrentTenant(r.Context(), func(ctx context.Context) error {
		if err := rs.EducationService.CreateGroup(ctx, group); err != nil {
			return err
		}
		// Assign teachers to the group if any were provided
		if len(req.TeacherIDs) > 0 {
			if err := rs.EducationService.UpdateGroupTeachers(ctx, group.ID, req.TeacherIDs); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// The route-level tenant transaction is the outer transaction. A 4xx
		// response would otherwise commit partial group/teacher writes.
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
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
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing group
	group, err := rs.EducationService.GetGroup(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New(common.MsgGroupNotFound)))
		return
	}

	// Update fields
	group.Name = req.Name
	group.RoomID = req.RoomID

	// Update group
	if err := tenant.WithinCurrentTenant(r.Context(), func(ctx context.Context) error {
		if err := rs.EducationService.UpdateGroup(ctx, group); err != nil {
			return err
		}
		// Update teacher assignments if provided
		if req.TeacherIDs != nil {
			if err := rs.EducationService.UpdateGroupTeachers(ctx, group.ID, req.TeacherIDs); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		// ErrorRenderer maps validation failures to 4xx; explicitly request a
		// rollback so a partial teacher-set mutation cannot commit with that
		// non-5xx response.
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, ErrorRenderer(err))
		return
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
	if err := tenant.WithinCurrentTenant(r.Context(), func(ctx context.Context) error {
		return rs.EducationService.DeleteGroup(ctx, id)
	}); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Gruppe kann nicht gelöscht werden: Gruppe wird noch in anderen Bereichen referenziert"))
			return
		}
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Group deleted successfully")
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
			FullName:       teacher.FullName(),
			CreatedAt:      teacher.CreatedAt,
			UpdatedAt:      teacher.UpdatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, responses, "Group supervisors retrieved successfully")
}
