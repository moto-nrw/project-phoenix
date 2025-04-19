package room

import (
	"fmt"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"
)

// API provides room management handlers.
type API struct {
	store RoomStore
}

// NewAPI configures and returns Room API.
func NewAPI(db *bun.DB) (*API, error) {
	store := NewRoomStore(db)
	api := &API{
		store: store,
	}
	return api, nil
}

// Router provides room routes.
func (a *API) Router() *chi.Mux {
	r := chi.NewRouter()

	// Room endpoints
	r.Get("/", a.handleGetRooms)
	r.Post("/", a.handleCreateRoom)
	r.Get("/grouped_by_category", a.handleGetRoomsGroupedByCategory)
	r.Get("/choose", a.handleGetRoomsForSelection)
	r.Get("/{id}", a.handleGetRoomByID)
	r.Put("/{id}", a.handleUpdateRoom)
	r.Delete("/{id}", a.handleDeleteRoom)
	r.Get("/{id}/current_occupancy", a.handleGetCurrentRoomOccupancy)
	r.Post("/{id}/register_tablet", a.handleRegisterTablet)
	r.Post("/{id}/unregister_tablet", a.handleUnregisterTablet)
	r.Get("/{id}/combined_group", a.handleGetCombinedGroupForRoom)

	// Combined groups endpoints
	r.Route("/combined_groups", func(r chi.Router) {
		r.Get("/", a.handleGetActiveCombinedGroups)
		r.Post("/merge", a.handleMergeRooms)
		r.Delete("/{id}", a.handleDeactivateCombinedGroup)
	})

	// Room occupancy endpoints
	r.Route("/occupancies", func(r chi.Router) {
		r.Get("/", a.handleGetAllRoomOccupancies)
		r.Get("/{id}", a.handleGetRoomOccupancyByID)
	})

	return r
}

// handleGetRooms returns all rooms, with optional filtering
func (a *API) handleGetRooms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	category := r.URL.Query().Get("category")
	building := r.URL.Query().Get("building")
	floorStr := r.URL.Query().Get("floor")
	occupiedStr := r.URL.Query().Get("occupied")

	var rooms []models.Room
	var err error

	// Apply filters if provided
	if category != "" {
		rooms, err = a.store.GetRoomsByCategory(ctx, category)
	} else if building != "" {
		rooms, err = a.store.GetRoomsByBuilding(ctx, building)
	} else if floorStr != "" {
		floor, convErr := strconv.Atoi(floorStr)
		if convErr != nil {
			render.Render(w, r, ErrInvalidRequest(convErr))
			return
		}
		rooms, err = a.store.GetRoomsByFloor(ctx, floor)
	} else if occupiedStr != "" {
		occupied, convErr := strconv.ParseBool(occupiedStr)
		if convErr != nil {
			render.Render(w, r, ErrInvalidRequest(convErr))
			return
		}
		rooms, err = a.store.GetRoomsByOccupied(ctx, occupied)
	} else {
		// No filters, get all rooms
		rooms, err = a.store.GetRooms(ctx)
	}

	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render response
	render.JSON(w, r, rooms)
}

// handleCreateRoom creates a new room
func (a *API) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	room := new(models.Room)

	// Parse request body
	if err := render.Bind(r, room); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Create room in database
	if err := a.store.CreateRoom(r.Context(), room); err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render created room
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, room)
}

// handleGetRoomByID returns a room by ID
func (a *API) handleGetRoomByID(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Get room from database
	room, err := a.store.GetRoomByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Render room
	render.JSON(w, r, room)
}

// handleUpdateRoom updates a room
func (a *API) handleUpdateRoom(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if room exists
	existingRoom, err := a.store.GetRoomByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Parse request body into existing room
	if err := render.Bind(r, existingRoom); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Update room in database
	if err := a.store.UpdateRoom(r.Context(), existingRoom); err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render updated room
	render.JSON(w, r, existingRoom)
}

// handleDeleteRoom deletes a room
func (a *API) handleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if room exists
	_, err = a.store.GetRoomByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Delete room from database
	if err := a.store.DeleteRoom(r.Context(), id); err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return success with no content
	w.WriteHeader(http.StatusNoContent)
}

// handleGetRoomsGroupedByCategory returns rooms grouped by category
func (a *API) handleGetRoomsGroupedByCategory(w http.ResponseWriter, r *http.Request) {
	// Get rooms grouped by category
	groupedRooms, err := a.store.GetRoomsGroupedByCategory(r.Context())
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Convert map to array for the expected API response format
	var response []map[string]interface{}
	for category, rooms := range groupedRooms {
		group := map[string]interface{}{
			"category": category,
			"rooms":    rooms,
		}
		response = append(response, group)
	}

	// Render response
	render.JSON(w, r, response)
}

// handleGetRoomsForSelection returns rooms formatted for selection UI
func (a *API) handleGetRoomsForSelection(w http.ResponseWriter, r *http.Request) {
	// This is a placeholder - would need more context about what "selection" means
	// For now, return all rooms with some additional data about occupancy
	rooms, err := a.store.GetRooms(r.Context())
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render response
	render.JSON(w, r, rooms)
}

// handleGetCurrentRoomOccupancy returns current occupancy for a room
func (a *API) handleGetCurrentRoomOccupancy(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Get current occupancy
	occupancy, err := a.store.GetCurrentRoomOccupancy(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Render response
	render.JSON(w, r, occupancy)
}

// handleRegisterTablet registers a tablet to a room
func (a *API) handleRegisterTablet(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if room exists
	_, err = a.store.GetRoomByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Parse request body
	req := new(RegisterTabletRequest)
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Register tablet
	occupancy, err := a.store.RegisterTablet(r.Context(), id, req)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render response
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, occupancy)
}

// handleUnregisterTablet unregisters a tablet from a room
func (a *API) handleUnregisterTablet(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Parse request body
	req := new(UnregisterTabletRequest)
	if err := render.Bind(r, req); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Unregister tablet
	if err := a.store.UnregisterTablet(r.Context(), id, req.DeviceID); err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return success
	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Tablet unregistered successfully",
	})
}

// MergeRoomsRequest represents the request payload for merging rooms
type MergeRoomsRequest struct {
	SourceRoomID int64      `json:"source_room_id"`
	TargetRoomID int64      `json:"target_room_id"`
	Name         string     `json:"name,omitempty"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	AccessPolicy string     `json:"access_policy,omitempty"`
}

// Bind preprocesses a MergeRoomsRequest
func (m *MergeRoomsRequest) Bind(r *http.Request) error {
	// Validate required fields
	if m.SourceRoomID == 0 {
		return fmt.Errorf("source_room_id is required")
	}
	if m.TargetRoomID == 0 {
		return fmt.Errorf("target_room_id is required")
	}

	// Validate that source and target are not the same
	if m.SourceRoomID == m.TargetRoomID {
		return fmt.Errorf("source and target rooms must be different")
	}

	// Validate access policy if provided
	if m.AccessPolicy != "" &&
		m.AccessPolicy != "all" &&
		m.AccessPolicy != "first" &&
		m.AccessPolicy != "specific" &&
		m.AccessPolicy != "manual" {
		return fmt.Errorf("access_policy must be one of: all, first, specific, manual")
	}

	return nil
}

// handleMergeRooms merges two rooms
func (a *API) handleMergeRooms(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	data := &MergeRoomsRequest{}
	if err := render.Bind(r, data); err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	ctx := r.Context()

	// Check if both rooms exist
	_, err := a.store.GetRoomByID(ctx, data.SourceRoomID)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("source room not found: %w", err)))
		return
	}

	_, err = a.store.GetRoomByID(ctx, data.TargetRoomID)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(fmt.Errorf("target room not found: %w", err)))
		return
	}

	// Perform the merge operation
	combinedGroup, err := a.store.MergeRooms(ctx, data.SourceRoomID, data.TargetRoomID, data.Name, data.ValidUntil, data.AccessPolicy)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return success response with combined group details
	response := map[string]interface{}{
		"success":        true,
		"message":        "Rooms merged successfully",
		"combined_group": combinedGroup,
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// handleGetAllRoomOccupancies returns all room occupancies
func (a *API) handleGetAllRoomOccupancies(w http.ResponseWriter, r *http.Request) {
	// Get all occupancies
	occupancies, err := a.store.GetAllRoomOccupancies(r.Context())
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render response
	render.JSON(w, r, occupancies)
}

// handleGetRoomOccupancyByID returns room occupancy by ID
func (a *API) handleGetRoomOccupancyByID(w http.ResponseWriter, r *http.Request) {
	// Parse occupancy ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Get occupancy
	occupancy, err := a.store.GetRoomOccupancyByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Render response
	render.JSON(w, r, occupancy)
}

// handleGetCombinedGroupForRoom returns the active combined group associated with a room
func (a *API) handleGetCombinedGroupForRoom(w http.ResponseWriter, r *http.Request) {
	// Parse room ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if room exists
	_, err = a.store.GetRoomByID(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Get combined group for the room
	combinedGroup, err := a.store.GetCombinedGroupForRoom(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrNotFound())
		return
	}

	// Render response
	render.JSON(w, r, combinedGroup)
}

// handleGetActiveCombinedGroups returns all active combined groups
func (a *API) handleGetActiveCombinedGroups(w http.ResponseWriter, r *http.Request) {
	// Get all active combined groups
	combinedGroups, err := a.store.FindActiveCombinedGroups(r.Context())
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Render response
	render.JSON(w, r, combinedGroups)
}

// handleDeactivateCombinedGroup deactivates a combined group
func (a *API) handleDeactivateCombinedGroup(w http.ResponseWriter, r *http.Request) {
	// Parse combined group ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Deactivate the combined group
	err = a.store.DeactivateCombinedGroup(r.Context(), id)
	if err != nil {
		render.Render(w, r, ErrInternalServer(err))
		return
	}

	// Return success with no content
	render.Status(r, http.StatusOK)
	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Combined group deactivated successfully",
	})
}
