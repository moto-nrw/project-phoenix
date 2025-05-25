package rooms

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	facilityService "github.com/moto-nrw/project-phoenix/services/facilities"
)

// Resource defines the rooms API resource
type Resource struct {
	FacilityService facilityService.Service
}

// NewResource creates a new rooms resource
func NewResource(facilityService facilityService.Service) *Resource {
	return &Resource{
		FacilityService: facilityService,
	}
}

// Router returns a configured router for room endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Public routes
	r.Group(func(r chi.Router) {
		// Read-only endpoints can be accessed without authentication if needed
		r.Get("/", rs.listRooms)
		r.Get("/{id}", rs.getRoom)
		r.Get("/by-category", rs.getRoomsByCategory)
		r.Get("/{id}/history", rs.getRoomHistory)
	})

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Write operations require specific permissions
		r.With(authorize.RequiresPermission(permissions.RoomsCreate)).Post("/", rs.createRoom)
		r.With(authorize.RequiresPermission(permissions.RoomsUpdate)).Put("/{id}", rs.updateRoom)
		r.With(authorize.RequiresPermission(permissions.RoomsDelete)).Delete("/{id}", rs.deleteRoom)

		// Advanced operations
		r.With(authorize.RequiresPermission(permissions.RoomsRead)).Get("/buildings", rs.getBuildingList)
		r.With(authorize.RequiresPermission(permissions.RoomsRead)).Get("/categories", rs.getCategoryList)
		r.With(authorize.RequiresPermission(permissions.RoomsRead)).Get("/available", rs.getAvailableRooms)
	})

	return r
}

// RoomRequest represents a room request payload
type RoomRequest struct {
	Name     string `json:"name"`
	Building string `json:"building,omitempty"`
	Floor    int    `json:"floor"`
	Capacity int    `json:"capacity"`
	Category string `json:"category,omitempty"`
	Color    string `json:"color,omitempty"`
}

// Bind validates the room request
func (req *RoomRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Name, validation.Required),
		validation.Field(&req.Capacity, validation.Min(0)),
	)
}

// RoomResponse represents a room response
type RoomResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Building  string    `json:"building,omitempty"`
	Floor     int       `json:"floor"`
	Capacity  int       `json:"capacity"`
	Category  string    `json:"category"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Convert a Room model to a RoomResponse
func newRoomResponse(room *facilities.Room) RoomResponse {
	return RoomResponse{
		ID:        room.ID,
		Name:      room.Name,
		Building:  room.Building,
		Floor:     room.Floor,
		Capacity:  room.Capacity,
		Category:  room.Category,
		Color:     room.Color,
		CreatedAt: room.CreatedAt,
		UpdatedAt: room.UpdatedAt,
	}
}

// listRooms handles listing all rooms
func (rs *Resource) listRooms(w http.ResponseWriter, r *http.Request) {
	// Create query options with filter
	queryOptions := base.NewQueryOptions()

	// Add filters if provided
	building := r.URL.Query().Get("building")
	category := r.URL.Query().Get("category")

	if building != "" {
		queryOptions.Filter.Equal("building", building)
	}

	if category != "" {
		queryOptions.Filter.Equal("category", category)
	}

	// Add pagination if provided
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

	// Get rooms from service
	rooms, err := rs.FacilityService.ListRooms(r.Context(), queryOptions)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response
	roomResponses := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		roomResponses[i] = newRoomResponse(room)
	}

	// Use common paginated response
	common.RespondWithPagination(w, r, http.StatusOK, roomResponses, page, pageSize, len(rooms), "Rooms retrieved successfully")
}

// getRoom handles getting a room by ID
func (rs *Resource) getRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get room from service
	room, err := rs.FacilityService.GetRoom(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("room not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, newRoomResponse(room), "Room retrieved successfully")
}

// createRoom handles creating a new room
func (rs *Resource) createRoom(w http.ResponseWriter, r *http.Request) {
	req := &RoomRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create room model
	room := &facilities.Room{
		Name:     req.Name,
		Building: req.Building,
		Floor:    req.Floor,
		Capacity: req.Capacity,
		Category: req.Category,
		Color:    req.Color,
	}

	// Validate the room
	if err := room.Validate(); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Create room using service
	if err := rs.FacilityService.CreateRoom(r.Context(), room); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return response
	common.Respond(w, r, http.StatusCreated, newRoomResponse(room), "Room created successfully")
}

// updateRoom handles updating an existing room
func (rs *Resource) updateRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get existing room
	room, err := rs.FacilityService.GetRoom(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, common.ErrorNotFound(errors.New("room not found"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &RoomRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Update room fields
	room.Name = req.Name
	room.Building = req.Building
	room.Floor = req.Floor
	room.Capacity = req.Capacity
	room.Category = req.Category
	room.Color = req.Color

	// Validate the room
	if err := room.Validate(); err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Update room using service
	if err := rs.FacilityService.UpdateRoom(r.Context(), room); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, newRoomResponse(room), "Room updated successfully")
}

// deleteRoom handles deleting a room
func (rs *Resource) deleteRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete room using service
	if err := rs.FacilityService.DeleteRoom(r.Context(), id); err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Return response
	common.RespondNoContent(w, r)
}

// getRoomsByCategory handles getting rooms by category
func (rs *Resource) getRoomsByCategory(w http.ResponseWriter, r *http.Request) {
	// Get category from query parameter
	category := r.URL.Query().Get("category")
	if category == "" {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("category parameter is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get rooms by category
	rooms, err := rs.FacilityService.FindRoomsByCategory(r.Context(), category)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response
	roomResponses := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		roomResponses[i] = newRoomResponse(room)
	}

	// Return response
	common.Respond(w, r, http.StatusOK, roomResponses, "Rooms retrieved successfully")
}

// getBuildingList handles getting a list of building names
func (rs *Resource) getBuildingList(w http.ResponseWriter, r *http.Request) {
	// Get buildings list
	buildings, err := rs.FacilityService.GetBuildingList(r.Context())
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, map[string][]string{"buildings": buildings}, "Building list retrieved successfully")
}

// getCategoryList handles getting a list of room categories
func (rs *Resource) getCategoryList(w http.ResponseWriter, r *http.Request) {
	// Get categories list
	categories, err := rs.FacilityService.GetCategoryList(r.Context())
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, map[string][]string{"categories": categories}, "Category list retrieved successfully")
}

// getAvailableRooms handles getting available rooms by capacity
func (rs *Resource) getAvailableRooms(w http.ResponseWriter, r *http.Request) {
	// Parse capacity from query parameter
	capacity := 0
	if capacityStr := r.URL.Query().Get("capacity"); capacityStr != "" {
		if cap, err := strconv.Atoi(capacityStr); err == nil && cap > 0 {
			capacity = cap
		}
	}

	// Get available rooms
	rooms, err := rs.FacilityService.GetAvailableRooms(r.Context(), capacity)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to response
	roomResponses := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		roomResponses[i] = newRoomResponse(room)
	}

	// Return response
	common.Respond(w, r, http.StatusOK, roomResponses, "Available rooms retrieved successfully")
}

// getRoomHistory handles getting the visit history for a room
func (rs *Resource) getRoomHistory(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInvalidRequest(errors.New("invalid room ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get time range from query parameters
	startTime := time.Now().AddDate(0, 0, -7) // Default to last 7 days
	endTime := time.Now()

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		parsedStart, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid start date format"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
		startTime = parsedStart
	}

	if endStr := r.URL.Query().Get("end"); endStr != "" {
		parsedEnd, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid end date format"))); err != nil {
				log.Printf("Error rendering error response: %v", err)
			}
			return
		}
		endTime = parsedEnd
	}

	// Validate date range
	if startTime.After(endTime) {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("start date must be before end date"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get room history from service
	history, err := rs.FacilityService.GetRoomHistory(r.Context(), id, startTime, endTime)
	if err != nil {
		if err := render.Render(w, r, common.ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, history, "Room history retrieved successfully")
}
