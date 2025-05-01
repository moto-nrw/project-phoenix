// Package group provides the group management API
package group

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/sirupsen/logrus"
)

// Resource defines the group management resource
type Resource struct {
	Store     GroupStore
	AuthStore AuthTokenStore
}

// GroupStore defines database operations for group management
type GroupStore interface {
	GetGroupByID(ctx context.Context, id int64) (*models2.Group, error)
	CreateGroup(ctx context.Context, group *models2.Group, supervisorIDs []int64) error
	UpdateGroup(ctx context.Context, group *models2.Group) error
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, filters map[string]interface{}) ([]models2.Group, error)
	UpdateGroupSupervisors(ctx context.Context, groupID int64, supervisorIDs []int64) error

	CreateCombinedGroup(ctx context.Context, combinedGroup *models2.CombinedGroup, groupIDs []int64, specialistIDs []int64) error
	GetCombinedGroupByID(ctx context.Context, id int64) (*models2.CombinedGroup, error)
	ListCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error)
	MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64) (*models2.CombinedGroup, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt.Token, error)
}

// NewResource creates a new group management resource
func NewResource(store GroupStore, authStore AuthTokenStore) *Resource {
	return &Resource{
		Store:     store,
		AuthStore: authStore,
	}
}

// Router creates a router for group management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt.Authenticator)

		// Group routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listGroups)
			r.Post("/", rs.createGroup)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getGroup)
				r.Put("/", rs.updateGroup)
				r.Delete("/", rs.deleteGroup)
				r.Post("/supervisors", rs.updateGroupSupervisors)
				r.Post("/representative", rs.setGroupRepresentative)
				r.Delete("/representative", rs.removeGroupRepresentative)
				r.Get("/students", rs.getGroupStudents)
			})
		})

		// Combined Group routes
		r.Route("/combined", func(r chi.Router) {
			r.Get("/", rs.listCombinedGroups)
			r.Post("/", rs.createCombinedGroup)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getCombinedGroup)
				r.Put("/", rs.updateCombinedGroup)
				r.Delete("/", rs.deleteCombinedGroup)
				r.Post("/groups", rs.addGroupsToCombinedGroup)
				r.Delete("/groups/{groupId}", rs.removeGroupFromCombinedGroup)
				r.Post("/specialists", rs.updateCombinedGroupSpecialists)
			})
		})

		// Special operations
		r.Post("/merge-rooms", rs.mergeRooms)
		r.Get("/room/{roomId}/group", rs.getGroupByRoomID)

		// Public operations (accessible without admin role)
		r.Route("/public", func(r chi.Router) {
			r.Get("/", rs.listGroupsPublic)
		})
	})

	return r
}

// ======== Request/Response Models ========

// GroupRequest is the request payload for Group data
type GroupRequest struct {
	*models2.Group
	SupervisorIDs []int64 `json:"supervisor_ids,omitempty"`
}

// Bind preprocesses a GroupRequest
func (req *GroupRequest) Bind(r *http.Request) error {
	if req.Group == nil {
		return errors.New("missing group data")
	}
	return nil
}

// SupervisorRequest is the request payload for updating group supervisors
type SupervisorRequest struct {
	SupervisorIDs []int64 `json:"supervisor_ids"`
}

// Bind preprocesses a SupervisorRequest
func (req *SupervisorRequest) Bind(r *http.Request) error {
	if req.SupervisorIDs == nil {
		return errors.New("supervisor_ids is required")
	}
	return nil
}

// RepresentativeRequest is the request payload for setting a group representative
type RepresentativeRequest struct {
	SpecialistID int64 `json:"specialist_id"`
}

// Bind preprocesses a RepresentativeRequest
func (req *RepresentativeRequest) Bind(r *http.Request) error {
	if req.SpecialistID == 0 {
		return errors.New("specialist_id is required")
	}
	return nil
}

// CombinedGroupRequest is the request payload for CombinedGroup data
type CombinedGroupRequest struct {
	*models2.CombinedGroup
	GroupIDs      []int64 `json:"group_ids,omitempty"`
	SpecialistIDs []int64 `json:"specialist_ids,omitempty"`
}

// Bind preprocesses a CombinedGroupRequest
func (req *CombinedGroupRequest) Bind(r *http.Request) error {
	if req.CombinedGroup == nil {
		return errors.New("missing combined group data")
	}
	return nil
}

// GroupIDsRequest is the request payload for adding groups to a combined group
type GroupIDsRequest struct {
	GroupIDs []int64 `json:"group_ids"`
}

// Bind preprocesses a GroupIDsRequest
func (req *GroupIDsRequest) Bind(r *http.Request) error {
	if req.GroupIDs == nil || len(req.GroupIDs) == 0 {
		return errors.New("group_ids is required")
	}
	return nil
}

// MergeRoomsRequest is the request payload for merging rooms
type MergeRoomsRequest struct {
	SourceRoomID int64 `json:"source_room_id"`
	TargetRoomID int64 `json:"target_room_id"`
}

// Bind preprocesses a MergeRoomsRequest
func (req *MergeRoomsRequest) Bind(r *http.Request) error {
	if req.SourceRoomID == 0 {
		return errors.New("source_room_id is required")
	}
	if req.TargetRoomID == 0 {
		return errors.New("target_room_id is required")
	}
	return nil
}

// ======== Group Handlers ========

// listGroups returns a list of all groups with optional filtering
func (rs *Resource) listGroups(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if supervisorIDStr := r.URL.Query().Get("supervisor_id"); supervisorIDStr != "" {
		supervisorID, err := strconv.ParseInt(supervisorIDStr, 10, 64)
		if err == nil {
			filters["supervisor_id"] = supervisorID
		} else {
			logger.WithError(err).Warn("Invalid supervisor_id parameter")
		}
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	groups, err := rs.Store.ListGroups(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to list groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("count", len(groups)).Info("Listed groups")
	render.JSON(w, r, groups)
}

// createGroup creates a new group
func (rs *Resource) createGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &GroupRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid group creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the group data
	if err := ValidateGroup(data.Group); err != nil {
		logger.WithError(err).Warn("Group validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateGroup(ctx, data.Group, data.SupervisorIDs); err != nil {
		logger.WithError(err).Error("Failed to create group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created group with all its relations
	group, err := rs.Store.GetGroupByID(ctx, data.Group.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"group_id": group.ID,
		"name":     group.Name,
	}).Info("Group created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, group)
}

// getGroup returns a specific group
func (rs *Resource) getGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, group)
}

// updateGroup updates a specific group
func (rs *Resource) updateGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &GroupRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid group update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the group data
	if err := ValidateGroup(data.Group); err != nil {
		logger.WithError(err).Warn("Group validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get group by ID for update")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update group fields, preserving what shouldn't be changed
	group.Name = data.Name
	group.RoomID = data.RoomID
	group.RepresentativeID = data.RepresentativeID

	if err := rs.Store.UpdateGroup(ctx, group); err != nil {
		logger.WithError(err).Error("Failed to update group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// If supervisor IDs were provided, update them
	if data.SupervisorIDs != nil {
		if err := rs.Store.UpdateGroupSupervisors(ctx, id, data.SupervisorIDs); err != nil {
			logger.WithError(err).Error("Failed to update group supervisors")
			render.Render(w, r, ErrInternalServerError(err))
			return
		}
	}

	// Get the updated group with all its relations
	updatedGroup, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("group_id", id).Info("Group updated successfully")
	render.JSON(w, r, updatedGroup)
}

// deleteGroup deletes a specific group
func (rs *Resource) deleteGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteGroup(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("group_id", id).Info("Group deleted successfully")
	render.NoContent(w, r)
}

// updateGroupSupervisors updates the supervisors for a group
func (rs *Resource) updateGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &SupervisorRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid supervisor update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Verify the group exists
	_, err = rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update supervisors
	if err := rs.Store.UpdateGroupSupervisors(ctx, id, data.SupervisorIDs); err != nil {
		logger.WithError(err).Error("Failed to update group supervisors")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated group with all its relations
	updatedGroup, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"group_id":       id,
		"supervisor_ids": data.SupervisorIDs,
	}).Info("Group supervisors updated successfully")

	render.JSON(w, r, updatedGroup)
}

// getGroupStudents returns all students belonging to a group
func (rs *Resource) getGroupStudents(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Return the students associated with the group
	// Now GetGroupByID loads the Students relation properly
	if group.Students == nil {
		group.Students = []models2.Student{}
		// Log that no students were found for this group
		logger.WithField("group_id", id).Info("No students found for group")
	} else {
		// Log how many students were found
		logger.WithFields(logrus.Fields{
			"group_id": id,
			"count":    len(group.Students),
		}).Info("Retrieved students for group")
	}

	render.JSON(w, r, group.Students)
}

// setGroupRepresentative sets a student as the representative for a group
func (rs *Resource) setGroupRepresentative(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &RepresentativeRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid representative request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update representative
	group.RepresentativeID = &data.SpecialistID

	if err := rs.Store.UpdateGroup(ctx, group); err != nil {
		logger.WithError(err).Error("Failed to update group representative")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated group with all its relations
	updatedGroup, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"group_id":          id,
		"representative_id": data.SpecialistID,
	}).Info("Group representative updated successfully")

	render.JSON(w, r, updatedGroup)
}

// removeGroupRepresentative removes the representative from a group
func (rs *Resource) removeGroupRepresentative(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Remove representative
	group.RepresentativeID = nil

	if err := rs.Store.UpdateGroup(ctx, group); err != nil {
		logger.WithError(err).Error("Failed to remove group representative")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated group with all its relations
	updatedGroup, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("group_id", id).Info("Group representative removed successfully")

	render.JSON(w, r, updatedGroup)
}

// getGroupByRoomID returns the group associated with a room
func (rs *Resource) getGroupByRoomID(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	roomIDStr := chi.URLParam(r, "roomId")
	roomID, err := strconv.ParseInt(roomIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid room ID format")))
		return
	}

	ctx := r.Context()

	// Get groups with filter for roomID
	filters := map[string]interface{}{
		"room_id": roomID,
	}
	groups, err := rs.Store.ListGroups(ctx, filters)
	if err != nil {
		logger.WithError(err).Error("Failed to get groups by room ID")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	if len(groups) == 0 {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Return the first group associated with this room
	render.JSON(w, r, groups[0])
}

// ======== Combined Group Handlers ========

// listCombinedGroups returns a list of all combined groups
func (rs *Resource) listCombinedGroups(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Parse query parameters for filtering
	activeOnly := false
	if activeStr := r.URL.Query().Get("active"); activeStr == "true" {
		activeOnly = true
	}

	combinedGroups, err := rs.Store.ListCombinedGroups(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to list combined groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Filter for active groups if requested
	if activeOnly {
		activeCombinedGroups := make([]models2.CombinedGroup, 0)
		for _, cg := range combinedGroups {
			if cg.IsActive {
				activeCombinedGroups = append(activeCombinedGroups, cg)
			}
		}
		combinedGroups = activeCombinedGroups
	}

	logger.WithField("count", len(combinedGroups)).Info("Listed combined groups")
	render.JSON(w, r, combinedGroups)
}

// createCombinedGroup creates a new combined group
func (rs *Resource) createCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &CombinedGroupRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid combined group creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the combined group data
	if err := ValidateCombinedGroup(data.CombinedGroup); err != nil {
		logger.WithError(err).Warn("Combined group validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateCombinedGroup(ctx, data.CombinedGroup, data.GroupIDs, data.SpecialistIDs); err != nil {
		logger.WithError(err).Error("Failed to create combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created combined group with all its relations
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, data.CombinedGroup.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"combined_group_id": combinedGroup.ID,
		"name":              combinedGroup.Name,
	}).Info("Combined group created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, combinedGroup)
}

// getCombinedGroup returns a specific combined group
func (rs *Resource) getCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, combinedGroup)
}

// updateCombinedGroup updates a specific combined group
func (rs *Resource) updateCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &CombinedGroupRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid combined group update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the combined group data
	if err := ValidateCombinedGroup(data.CombinedGroup); err != nil {
		logger.WithError(err).Warn("Combined group validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group by ID for update")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update combined group fields
	combinedGroup.Name = data.Name
	combinedGroup.IsActive = data.IsActive
	combinedGroup.ValidUntil = data.ValidUntil
	combinedGroup.AccessPolicy = data.AccessPolicy
	combinedGroup.SpecificGroupID = data.SpecificGroupID

	// Custom update method would be needed here
	// For now, we'll re-create the combined group with the same ID
	if err := rs.Store.CreateCombinedGroup(ctx, combinedGroup, data.GroupIDs, data.SpecialistIDs); err != nil {
		logger.WithError(err).Error("Failed to update combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated combined group with all its relations
	updatedCombinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("combined_group_id", id).Info("Combined group updated successfully")
	render.JSON(w, r, updatedCombinedGroup)
}

// deleteCombinedGroup deletes a specific combined group
func (rs *Resource) deleteCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	// This would need a custom delete method in the GroupStore interface
	// For now, we'll mark it as inactive instead
	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Mark as inactive
	combinedGroup.IsActive = false
	now := time.Now()
	combinedGroup.ValidUntil = &now

	// Custom update method would be needed here
	// For now, we'll re-create the combined group with the same groups and specialists
	var groupIDs []int64
	for _, group := range combinedGroup.Groups {
		groupIDs = append(groupIDs, group.ID)
	}

	var specialistIDs []int64
	for _, specialist := range combinedGroup.AccessSpecialists {
		specialistIDs = append(specialistIDs, specialist.ID)
	}

	if err := rs.Store.CreateCombinedGroup(ctx, combinedGroup, groupIDs, specialistIDs); err != nil {
		logger.WithError(err).Error("Failed to update combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("combined_group_id", id).Info("Combined group deactivated successfully")
	render.NoContent(w, r)
}

// addGroupsToCombinedGroup adds groups to a combined group
func (rs *Resource) addGroupsToCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &GroupIDsRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid group IDs request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Add the new groups to the existing groups
	var existingGroupIDs []int64
	for _, group := range combinedGroup.Groups {
		existingGroupIDs = append(existingGroupIDs, group.ID)
	}

	// Merge the existing and new group IDs
	allGroupIDs := mergeUniqueIDs(existingGroupIDs, data.GroupIDs)

	// Get existing specialist IDs
	var specialistIDs []int64
	for _, specialist := range combinedGroup.AccessSpecialists {
		specialistIDs = append(specialistIDs, specialist.ID)
	}

	// Update the combined group with the merged groups
	if err := rs.Store.CreateCombinedGroup(ctx, combinedGroup, allGroupIDs, specialistIDs); err != nil {
		logger.WithError(err).Error("Failed to add groups to combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated combined group
	updatedCombinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"combined_group_id": id,
		"added_group_ids":   data.GroupIDs,
	}).Info("Groups added to combined group successfully")

	render.JSON(w, r, updatedCombinedGroup)
}

// removeGroupFromCombinedGroup removes a group from a combined group
func (rs *Resource) removeGroupFromCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	groupIDStr := chi.URLParam(r, "groupId")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid group ID format")))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Remove the specified group ID from the existing groups
	var filteredGroupIDs []int64
	for _, group := range combinedGroup.Groups {
		if group.ID != groupID {
			filteredGroupIDs = append(filteredGroupIDs, group.ID)
		}
	}

	// If no groups were removed, return an error
	if len(filteredGroupIDs) == len(combinedGroup.Groups) {
		logger.WithFields(logrus.Fields{
			"combined_group_id": id,
			"group_id":          groupID,
		}).Warn("Group not found in combined group")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Get existing specialist IDs
	var specialistIDs []int64
	for _, specialist := range combinedGroup.AccessSpecialists {
		specialistIDs = append(specialistIDs, specialist.ID)
	}

	// Update the combined group with the filtered groups
	if err := rs.Store.CreateCombinedGroup(ctx, combinedGroup, filteredGroupIDs, specialistIDs); err != nil {
		logger.WithError(err).Error("Failed to remove group from combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated combined group
	updatedCombinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"combined_group_id": id,
		"removed_group_id":  groupID,
	}).Info("Group removed from combined group successfully")

	render.JSON(w, r, updatedCombinedGroup)
}

// updateCombinedGroupSpecialists updates the specialists with access to a combined group
func (rs *Resource) updateCombinedGroupSpecialists(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &SupervisorRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid specialist update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group by ID")
		render.Render(w, r, ErrNotFound)
		return
	}

	// Get existing group IDs
	var groupIDs []int64
	for _, group := range combinedGroup.Groups {
		groupIDs = append(groupIDs, group.ID)
	}

	// Update the combined group with the new specialists
	if err := rs.Store.CreateCombinedGroup(ctx, combinedGroup, groupIDs, data.SupervisorIDs); err != nil {
		logger.WithError(err).Error("Failed to update combined group specialists")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated combined group
	updatedCombinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve updated combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"combined_group_id": id,
		"specialist_ids":    data.SupervisorIDs,
	}).Info("Combined group specialists updated successfully")

	render.JSON(w, r, updatedCombinedGroup)
}

// ======== Special Operations ========

// mergeRooms merges two rooms and creates a combined group
func (rs *Resource) mergeRooms(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)

	data := &MergeRoomsRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid merge rooms request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.MergeRooms(ctx, data.SourceRoomID, data.TargetRoomID)
	if err != nil {
		logger.WithError(err).Error("Failed to merge rooms")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"source_room_id":    data.SourceRoomID,
		"target_room_id":    data.TargetRoomID,
		"combined_group_id": combinedGroup.ID,
	}).Info("Rooms merged successfully")

	// Return successful response with combined group
	render.JSON(w, r, map[string]interface{}{
		"success":        true,
		"message":        "Rooms merged successfully",
		"combined_group": combinedGroup,
	})
}

// listGroupsPublic returns a public list of all groups
func (rs *Resource) listGroupsPublic(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	groups, err := rs.Store.ListGroups(ctx, nil)
	if err != nil {
		logger.WithError(err).Error("Failed to list groups for public view")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Create a limited view of the groups for public access
	type PublicGroup struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	publicGroups := make([]PublicGroup, len(groups))
	for i, group := range groups {
		publicGroups[i] = PublicGroup{
			ID:   group.ID,
			Name: group.Name,
		}
	}

	logger.WithField("count", len(publicGroups)).Info("Listed public groups")
	render.JSON(w, r, publicGroups)
}

// ======== Helper Functions ========

// mergeUniqueIDs merges two slices of int64 and returns a slice with unique IDs
func mergeUniqueIDs(existingIDs, newIDs []int64) []int64 {
	// Create a map to track unique IDs
	seen := make(map[int64]bool)

	// Create result slice preserving the original order
	result := make([]int64, 0, len(existingIDs)+len(newIDs))

	// Add existing IDs first, maintaining their order
	for _, id := range existingIDs {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	// Add new IDs, maintaining their order
	for _, id := range newIDs {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}

	return result
}
