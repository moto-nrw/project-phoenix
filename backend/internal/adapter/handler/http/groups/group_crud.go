package groups

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

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
			logger.Logger.WithError(err).WithField("group_id", group.ID).Warn("Failed to get teachers for group")
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
		logger.Logger.WithError(err).WithField("group_id", id).Warn("Failed to get teachers for group")
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
			logger.Logger.WithError(err).WithField("group_id", group.ID).Warn("Failed to assign teachers to group")
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
			logger.Logger.WithError(err).WithField("group_id", group.ID).Warn("Error updating group teachers")
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
