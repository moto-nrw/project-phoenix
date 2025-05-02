// Package room provides the room management API
package room

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/uptrace/bun"
)

// Resource defines the room management resource
type Resource struct {
	Store RoomStore
}

// RoomStore defines database operations for room management
type RoomStore interface {
	// Room CRUD operations
	GetRooms(ctx context.Context) ([]models2.Room, error)
	GetRoomsByCategory(ctx context.Context, category string) ([]models2.Room, error)
	GetRoomsByBuilding(ctx context.Context, building string) ([]models2.Room, error)
	GetRoomsByFloor(ctx context.Context, floor int) ([]models2.Room, error)
	GetRoomsByOccupied(ctx context.Context, occupied bool) ([]models2.Room, error)
	GetRoomByID(ctx context.Context, id int64) (*models2.Room, error)
	CreateRoom(ctx context.Context, room *models2.Room) error
	UpdateRoom(ctx context.Context, room *models2.Room) error
	DeleteRoom(ctx context.Context, id int64) error
	GetRoomsGroupedByCategory(ctx context.Context) (map[string][]models2.Room, error)

	// Room occupancy operations
	GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*models2.RoomOccupancy, error)
	RegisterTablet(ctx context.Context, roomID int64, deviceID string, agID *int64, groupID *int64) (*models2.RoomOccupancy, error)
	UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error

	// Combined group operations
	MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models2.CombinedGroup, error)
	GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models2.CombinedGroup, error)
	FindActiveCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error)
	DeactivateCombinedGroup(ctx context.Context, id int64) error

	// Room history operations
	GetRoomHistoryByRoom(ctx context.Context, roomID int64) ([]models2.RoomHistory, error)
	GetRoomHistoryByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models2.RoomHistory, error)
	GetRoomHistoryBySupervisor(ctx context.Context, supervisorID int64) ([]models2.RoomHistory, error)
}

// NewResource creates a new room management resource
func NewResource(store RoomStore) *Resource {
	return &Resource{
		Store: store,
	}
}

// Router creates a router for room management
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()

	// JWT protected routes
	r.Group(func(r chi.Router) {
		r.Use(jwt.Authenticator)

		// Room routes
		r.Route("/", func(r chi.Router) {
			r.Get("/", rs.listRooms)
			r.Post("/", rs.createRoom)
			r.Get("/by-category", rs.getRoomsByCategory)
			r.Get("/by-building", rs.getRoomsByBuilding)
			r.Get("/by-floor", rs.getRoomsByFloor)
			r.Get("/by-occupied", rs.getRoomsByOccupied)
			r.Get("/grouped", rs.getRoomsGroupedByCategory)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", rs.getRoom)
				r.Put("/", rs.updateRoom)
				r.Delete("/", rs.deleteRoom)
				r.Get("/occupancy", rs.getRoomOccupancy)
				r.Post("/register", rs.registerTablet)
				r.Post("/unregister", rs.unregisterTablet)
				r.Get("/history", rs.getRoomHistory)
				r.Get("/combined-group", rs.getCombinedGroupForRoom)
			})
		})

		// Combined group routes
		r.Route("/combined-groups", func(r chi.Router) {
			r.Get("/", rs.getActiveCombinedGroups)
			r.Post("/merge", rs.mergeRooms)
			r.Delete("/{id}", rs.deactivateCombinedGroup)
		})

		// History routes
		r.Route("/history", func(r chi.Router) {
			r.Get("/by-date-range", rs.getRoomHistoryByDateRange)
			r.Get("/by-supervisor/{id}", rs.getRoomHistoryBySupervisor)
		})
	})

	return r
}

// ======== Room Handlers ========

// listRooms returns a list of all rooms
func (rs *Resource) listRooms(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	rooms, err := rs.Store.GetRooms(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to list rooms")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, rooms)
}

// createRoom creates a new room
func (rs *Resource) createRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	data := &RoomRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid room creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the room data
	if err := ValidateRoom(data.Room); err != nil {
		logger.WithError(err).Warn("Room validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	if err := rs.Store.CreateRoom(ctx, data.Room); err != nil {
		logger.WithError(err).Error("Failed to create room")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("room_id", data.Room.ID).Info("Room created successfully")
	render.Status(r, http.StatusCreated)
	render.JSON(w, r, data.Room)
}

// getRoom returns a specific room
func (rs *Resource) getRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	room, err := rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room by ID")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, room)
}

// updateRoom updates a specific room
func (rs *Resource) updateRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &RoomRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid room update request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Validate the room data
	if err := ValidateRoom(data.Room); err != nil {
		logger.WithError(err).Warn("Room validation failed")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Make sure ID in URL matches ID in body
	data.Room.ID = id

	room, err := rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room by ID for update")
		render.Render(w, r, ErrNotFound())
		return
	}

	// Update room fields
	room.RoomName = data.Room.RoomName
	room.Building = data.Room.Building
	room.Floor = data.Room.Floor
	room.Capacity = data.Room.Capacity
	room.Category = data.Room.Category
	room.Color = data.Room.Color

	if err := rs.Store.UpdateRoom(ctx, room); err != nil {
		logger.WithError(err).Error("Failed to update room")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("room_id", id).Info("Room updated successfully")
	render.JSON(w, r, room)
}

// deleteRoom deletes a specific room
func (rs *Resource) deleteRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	// Check if room exists
	_, err = rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room by ID for deletion")
		render.Render(w, r, ErrNotFound())
		return
	}

	if err := rs.Store.DeleteRoom(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to delete room")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("room_id", id).Info("Room deleted successfully")
	render.NoContent(w, r)
}

// getRoomsByCategory returns rooms by category
func (rs *Resource) getRoomsByCategory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	category := r.URL.Query().Get("category")
	if category == "" {
		logger.Warn("Missing category parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("category parameter is required")))
		return
	}

	rooms, err := rs.Store.GetRoomsByCategory(ctx, category)
	if err != nil {
		logger.WithError(err).Error("Failed to get rooms by category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, rooms)
}

// getRoomsByBuilding returns rooms by building
func (rs *Resource) getRoomsByBuilding(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	building := r.URL.Query().Get("building")
	if building == "" {
		logger.Warn("Missing building parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("building parameter is required")))
		return
	}

	rooms, err := rs.Store.GetRoomsByBuilding(ctx, building)
	if err != nil {
		logger.WithError(err).Error("Failed to get rooms by building")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, rooms)
}

// getRoomsByFloor returns rooms by floor
func (rs *Resource) getRoomsByFloor(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	floorStr := r.URL.Query().Get("floor")
	if floorStr == "" {
		logger.Warn("Missing floor parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("floor parameter is required")))
		return
	}

	floor, err := strconv.Atoi(floorStr)
	if err != nil {
		logger.WithError(err).Warn("Invalid floor format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid floor format")))
		return
	}

	rooms, err := rs.Store.GetRoomsByFloor(ctx, floor)
	if err != nil {
		logger.WithError(err).Error("Failed to get rooms by floor")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, rooms)
}

// getRoomsByOccupied returns rooms by occupied status
func (rs *Resource) getRoomsByOccupied(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	occupiedStr := r.URL.Query().Get("occupied")
	if occupiedStr == "" {
		logger.Warn("Missing occupied parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("occupied parameter is required")))
		return
	}

	occupied, err := strconv.ParseBool(occupiedStr)
	if err != nil {
		logger.WithError(err).Warn("Invalid occupied format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid occupied format")))
		return
	}

	rooms, err := rs.Store.GetRoomsByOccupied(ctx, occupied)
	if err != nil {
		logger.WithError(err).Error("Failed to get rooms by occupied status")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, rooms)
}

// getRoomsGroupedByCategory returns rooms grouped by category
func (rs *Resource) getRoomsGroupedByCategory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	groupedRooms, err := rs.Store.GetRoomsGroupedByCategory(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to get rooms grouped by category")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Format response for better readability
	response := make([]map[string]interface{}, 0)
	for category, rooms := range groupedRooms {
		response = append(response, map[string]interface{}{
			"category": category,
			"rooms":    rooms,
		})
	}

	render.JSON(w, r, response)
}

// ======== Room Occupancy Handlers ========

// getRoomOccupancy returns the current occupancy of a room
func (rs *Resource) getRoomOccupancy(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	occupancy, err := rs.Store.GetCurrentRoomOccupancy(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room occupancy")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, occupancy)
}

// registerTablet registers a tablet to a room
func (rs *Resource) registerTablet(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &RegisterTabletRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid register tablet request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if room exists
	_, err = rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room by ID for tablet registration")
		render.Render(w, r, ErrNotFound())
		return
	}

	occupancy, err := rs.Store.RegisterTablet(ctx, id, data.DeviceID, data.AgID, data.GroupID)
	if err != nil {
		if err.Error() == "tablet is already registered" {
			logger.WithError(err).Warn("Tablet already registered")
			render.Render(w, r, ErrTabletAlreadyRegistered())
		} else {
			logger.WithError(err).Error("Failed to register tablet")
			render.Render(w, r, ErrInternalServerError(err))
		}
		return
	}

	logger.WithFields(map[string]interface{}{
		"room_id":   id,
		"device_id": data.DeviceID,
	}).Info("Tablet registered successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, occupancy)
}

// unregisterTablet unregisters a tablet from a room
func (rs *Resource) unregisterTablet(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	data := &UnregisterTabletRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid unregister tablet request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if room exists
	_, err = rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room by ID for tablet unregistration")
		render.Render(w, r, ErrNotFound())
		return
	}

	if err := rs.Store.UnregisterTablet(ctx, id, data.DeviceID); err != nil {
		logger.WithError(err).Error("Failed to unregister tablet")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(map[string]interface{}{
		"room_id":   id,
		"device_id": data.DeviceID,
	}).Info("Tablet unregistered successfully")

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Tablet unregistered successfully",
	})
}

// ======== Combined Group Handlers ========

// mergeRooms merges two rooms together
func (rs *Resource) mergeRooms(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	data := &MergeRoomsRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid merge rooms request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if source room exists
	_, err := rs.Store.GetRoomByID(ctx, data.SourceRoomID)
	if err != nil {
		logger.WithError(err).Error("Source room not found")
		render.Render(w, r, ErrNotFound())
		return
	}

	// Check if target room exists
	_, err = rs.Store.GetRoomByID(ctx, data.TargetRoomID)
	if err != nil {
		logger.WithError(err).Error("Target room not found")
		render.Render(w, r, ErrNotFound())
		return
	}

	combinedGroup, err := rs.Store.MergeRooms(ctx, data.SourceRoomID, data.TargetRoomID, data.Name, data.ValidUntil, data.AccessPolicy)
	if err != nil {
		logger.WithError(err).Error("Failed to merge rooms")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(map[string]interface{}{
		"source_room_id":    data.SourceRoomID,
		"target_room_id":    data.TargetRoomID,
		"combined_group_id": combinedGroup.ID,
	}).Info("Rooms merged successfully")

	response := map[string]interface{}{
		"success":        true,
		"message":        "Rooms merged successfully",
		"combined_group": combinedGroup,
	}

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, response)
}

// getCombinedGroupForRoom returns the combined group for a room
func (rs *Resource) getCombinedGroupForRoom(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	// Check if room exists
	_, err = rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Room not found")
		render.Render(w, r, ErrNotFound())
		return
	}

	combinedGroup, err := rs.Store.GetCombinedGroupForRoom(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get combined group for room")
		render.Render(w, r, ErrNotFound())
		return
	}

	render.JSON(w, r, combinedGroup)
}

// getActiveCombinedGroups returns all active combined groups
func (rs *Resource) getActiveCombinedGroups(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	combinedGroups, err := rs.Store.FindActiveCombinedGroups(ctx)
	if err != nil {
		logger.WithError(err).Error("Failed to get active combined groups")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, combinedGroups)
}

// deactivateCombinedGroup deactivates a combined group
func (rs *Resource) deactivateCombinedGroup(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid combined group ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	if err := rs.Store.DeactivateCombinedGroup(ctx, id); err != nil {
		logger.WithError(err).Error("Failed to deactivate combined group")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("combined_group_id", id).Info("Combined group deactivated successfully")

	render.JSON(w, r, map[string]interface{}{
		"success": true,
		"message": "Combined group deactivated successfully",
	})
}

// ======== Room History Handlers ========

// getRoomHistory returns history for a specific room
func (rs *Resource) getRoomHistory(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid room ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	// Check if room exists
	_, err = rs.Store.GetRoomByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Room not found")
		render.Render(w, r, ErrNotFound())
		return
	}

	history, err := rs.Store.GetRoomHistoryByRoom(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room history")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, history)
}

// getRoomHistoryByDateRange returns room history within a date range
func (rs *Resource) getRoomHistoryByDateRange(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	if startDateStr == "" || endDateStr == "" {
		logger.Warn("Missing start_date or end_date parameter")
		render.Render(w, r, ErrInvalidRequest(errors.New("start_date and end_date parameters are required")))
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		logger.WithError(err).Warn("Invalid start_date format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid start_date format, use YYYY-MM-DD")))
		return
	}

	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		logger.WithError(err).Warn("Invalid end_date format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid end_date format, use YYYY-MM-DD")))
		return
	}

	// Add one day to end date to include the entire end date
	endDate = endDate.Add(24 * time.Hour)

	history, err := rs.Store.GetRoomHistoryByDateRange(ctx, startDate, endDate)
	if err != nil {
		logger.WithError(err).Error("Failed to get room history by date range")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, history)
}

// getRoomHistoryBySupervisor returns room history for a specific supervisor
func (rs *Resource) getRoomHistoryBySupervisor(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		logger.WithError(err).Warn("Invalid supervisor ID format")
		render.Render(w, r, ErrInvalidRequest(errors.New("invalid ID format")))
		return
	}

	history, err := rs.Store.GetRoomHistoryBySupervisor(ctx, id)
	if err != nil {
		logger.WithError(err).Error("Failed to get room history by supervisor")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	render.JSON(w, r, history)
}

// ValidateRoom validates the room data for API requests
func ValidateRoom(room *models2.Room) error {
	if room.RoomName == "" {
		return errors.New("room_name is required")
	}

	if room.Capacity < 0 {
		return errors.New("capacity must be non-negative")
	}

	return nil
}
