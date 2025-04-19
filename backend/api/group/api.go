package group

import (
	"context"
	"errors"
	jwt2 "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// Resource defines the group management resource
type Resource struct {
	Store     GroupStore
	AuthStore AuthTokenStore
}

// GroupStore defines database operations for group management
type GroupStore interface {
	GetGroupByID(ctx context.Context, id int64) (*models.Group, error)
	CreateGroup(ctx context.Context, group *models.Group, supervisorIDs []int64) error
	UpdateGroup(ctx context.Context, group *models.Group) error
	DeleteGroup(ctx context.Context, id int64) error
	ListGroups(ctx context.Context, filters map[string]interface{}) ([]models.Group, error)
	UpdateGroupSupervisors(ctx context.Context, groupID int64, supervisorIDs []int64) error
	CreateCombinedGroup(ctx context.Context, combinedGroup *models.CombinedGroup, groupIDs []int64, specialistIDs []int64) error
	GetCombinedGroupByID(ctx context.Context, id int64) (*models.CombinedGroup, error)
	ListCombinedGroups(ctx context.Context) ([]models.CombinedGroup, error)
	MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64) (*models.CombinedGroup, error)
}

// AuthTokenStore defines operations for the auth token store
type AuthTokenStore interface {
	GetToken(t string) (*jwt2.Token, error)
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
		r.Use(jwt2.Authenticator)

		// Group routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listGroups)
			r.Post("/", rs.createGroup)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getGroup)
				r.Put("/", rs.updateGroup)
				r.Delete("/", rs.deleteGroup)
				r.Post("/supervisors", rs.updateGroupSupervisors)
			})
		})

		// Combined Group routes
		r.Route("/combined", func(r chi.Router) {
			r.Get("/", rs.listCombinedGroups)
			r.Post("/", rs.createCombinedGroup)
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getCombinedGroup)
			})
		})

		// Special operations
		r.Post("/merge-rooms", rs.mergeRooms)
	})

	return r
}

// GroupRequest is the request payload for Group data
type GroupRequest struct {
	*models.Group
	SupervisorIDs []int64 `json:"supervisor_ids,omitempty"`
}

// Bind preprocesses a GroupRequest
func (req *GroupRequest) Bind(r *http.Request) error {
	return nil
}

// SupervisorRequest is the request payload for updating group supervisors
type SupervisorRequest struct {
	SupervisorIDs []int64 `json:"supervisor_ids"`
}

// Bind preprocesses a SupervisorRequest
func (req *SupervisorRequest) Bind(r *http.Request) error {
	return nil
}

// CombinedGroupRequest is the request payload for CombinedGroup data
type CombinedGroupRequest struct {
	*models.CombinedGroup
	GroupIDs      []int64 `json:"group_ids,omitempty"`
	SpecialistIDs []int64 `json:"specialist_ids,omitempty"`
}

// Bind preprocesses a CombinedGroupRequest
func (req *CombinedGroupRequest) Bind(r *http.Request) error {
	return nil
}

// MergeRoomsRequest is the request payload for merging rooms
type MergeRoomsRequest struct {
	SourceRoomID int64 `json:"source_room_id"`
	TargetRoomID int64 `json:"target_room_id"`
}

// Bind preprocesses a MergeRoomsRequest
func (req *MergeRoomsRequest) Bind(r *http.Request) error {
	return nil
}

// ======== Group Handlers ========

// listGroups returns a list of all groups with optional filtering
func (rs *Resource) listGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	filters := make(map[string]interface{})

	if supervisorIDStr := r.URL.Query().Get("supervisor_id"); supervisorIDStr != "" {
		supervisorID, err := strconv.ParseInt(supervisorIDStr, 10, 64)
		if err == nil {
			filters["supervisor_id"] = supervisorID
		}
	}

	if searchTerm := r.URL.Query().Get("search"); searchTerm != "" {
		filters["search"] = searchTerm
	}

	groups, err := rs.Store.ListGroups(ctx, filters)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, groups)
}

// createGroup creates a new group
func (rs *Resource) createGroup(w http.ResponseWriter, r *http.Request) {
	data := &GroupRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateGroup(ctx, data.Group, data.SupervisorIDs); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created group with all its relations
	group, err := rs.Store.GetGroupByID(ctx, data.Group.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, group)
}

// getGroup returns a specific group
func (rs *Resource) getGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, group)
}

// updateGroup updates a specific group
func (rs *Resource) updateGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &GroupRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	group, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	// Update group fields, preserving what shouldn't be changed
	group.Name = data.Name
	group.RoomID = data.RoomID
	group.RepresentativeID = data.RepresentativeID

	if err := rs.Store.UpdateGroup(ctx, group); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// If supervisor IDs were provided, update them
	if data.SupervisorIDs != nil {
		if err := rs.Store.UpdateGroupSupervisors(ctx, group.ID, data.SupervisorIDs); err != nil {
			render.Render(w, r, ErrInternalServerError(err))
			return
		}
	}

	// Get the updated group with all its relations
	updatedGroup, err := rs.Store.GetGroupByID(ctx, group.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, updatedGroup)
}

// deleteGroup deletes a specific group
func (rs *Resource) deleteGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	if err := rs.Store.DeleteGroup(ctx, id); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.NoContent(w, r)
}

// updateGroupSupervisors updates the supervisors for a group
func (rs *Resource) updateGroupSupervisors(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &SupervisorRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Update supervisors
	if err := rs.Store.UpdateGroupSupervisors(ctx, id, data.SupervisorIDs); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the updated group with all its relations
	updatedGroup, err := rs.Store.GetGroupByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, updatedGroup)
}

// ======== Combined Group Handlers ========

// listCombinedGroups returns a list of all combined groups
func (rs *Resource) listCombinedGroups(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	combinedGroups, err := rs.Store.ListCombinedGroups(ctx)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, combinedGroups)
}

// createCombinedGroup creates a new combined group
func (rs *Resource) createCombinedGroup(w http.ResponseWriter, r *http.Request) {
	data := &CombinedGroupRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	if err := rs.Store.CreateCombinedGroup(ctx, data.CombinedGroup, data.GroupIDs, data.SpecialistIDs); err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the created combined group with all its relations
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, data.CombinedGroup.ID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, combinedGroup)
}

// getCombinedGroup returns a specific combined group
func (rs *Resource) getCombinedGroup(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.GetCombinedGroupByID(ctx, id)
	if err != nil {
		render.Render(w, r, ErrNotFound)
		return
	}

	render.JSON(w, r, combinedGroup)
}

// ======== Special Operations ========

// mergeRooms merges two rooms and creates a combined group
func (rs *Resource) mergeRooms(w http.ResponseWriter, r *http.Request) {
	data := &MergeRoomsRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()
	combinedGroup, err := rs.Store.MergeRooms(ctx, data.SourceRoomID, data.TargetRoomID)
	if err != nil {
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Return successful response with combined group
	render.JSON(w, r, map[string]interface{}{
		"success":        true,
		"message":        "Rooms merged successfully",
		"combined_group": combinedGroup,
	})
}
