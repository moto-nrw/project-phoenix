// Package rooms serves the established /api/rooms contract through the
// Facilities capability. Authentication, rendering, cross-owner projections,
// and tenant settings are supplied by the composition root.
package rooms

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

type Middleware = func(http.Handler) http.Handler

type FailureKind string

const (
	FailureInvalid   FailureKind = "invalid"
	FailureForbidden FailureKind = "forbidden"
	FailureNotFound  FailureKind = "not_found"
	FailureConflict  FailureKind = "conflict"
	FailureInternal  FailureKind = "internal"
)

type Pagination struct {
	Page     int
	PageSize int
	Total    int
}

// RoomView adds the live-presence projection to the owner-controlled room.
type RoomView struct {
	facilities.Room
	IsOccupied      bool
	GroupName       *string
	CategoryName    *string
	StudentCount    int
	SupervisorNames *string
}

type RoomSessionEntry struct {
	SessionID       int64      `json:"session_id"`
	StartedAt       time.Time  `json:"started_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationMinutes *int       `json:"duration_minutes,omitempty"`
	ActivityName    string     `json:"activity_name"`
	SupervisorName  string     `json:"supervisor_name"`
	StudentCount    int        `json:"student_count"`
}

type SnapshotRequest struct {
	Format         string
	Title          string
	RoomIDs        *[]int64
	IncludeTransit bool
}

type ExportFile struct {
	Data        []byte
	ContentType string
	Filename    string
}

// Runtime contains the cross-owner and HTTP-platform behavior. All closures
// are required so missing production wiring fails during startup.
type Runtime struct {
	Protected  func(chi.Router, func(chi.Router, Middleware))
	Permission func(string) Middleware
	ParseID    func(*http.Request) (int64, error)
	Pagination func(*http.Request) (int, int)
	Success    func(http.ResponseWriter, *http.Request, int, any, string)
	Paginated  func(http.ResponseWriter, *http.Request, int, any, Pagination, string)
	NoContent  func(http.ResponseWriter, *http.Request)
	Failure    func(http.ResponseWriter, *http.Request, FailureKind, error, string)
	Read       string
	ReadUsers  string
	Create     string
	Update     string
	Delete     string

	Occupancy        func(context.Context, []facilities.Room) ([]RoomView, error)
	HistoryConfig    func(context.Context) (context.Context, bool, int)
	HistoryAllowed   func(context.Context) (bool, error)
	History          func(context.Context, int64, time.Time, time.Time) ([]RoomSessionEntry, error)
	TodayBounds      func() (time.Time, time.Time)
	ExportSnapshot   func(context.Context, SnapshotRequest) (ExportFile, error)
	ConstraintFailed func(error) bool
	Log              *slog.Logger
}

type Resource struct {
	rooms   facilities.Capability
	runtime Runtime
}

func NewResource(rooms facilities.Capability, runtime Runtime) *Resource {
	if rooms == nil || runtime.Protected == nil || runtime.Permission == nil ||
		runtime.ParseID == nil || runtime.Pagination == nil || runtime.Success == nil ||
		runtime.Paginated == nil || runtime.NoContent == nil || runtime.Failure == nil ||
		runtime.Occupancy == nil || runtime.HistoryConfig == nil || runtime.HistoryAllowed == nil ||
		runtime.History == nil || runtime.TodayBounds == nil || runtime.ExportSnapshot == nil ||
		runtime.ConstraintFailed == nil || runtime.Log == nil || runtime.Read == "" ||
		runtime.ReadUsers == "" || runtime.Create == "" || runtime.Update == "" || runtime.Delete == "" {
		panic("rooms HTTP: all dependencies are required")
	}
	return &Resource{rooms: rooms, runtime: runtime}
}

func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/", rs.listRooms)
		protected.With(rs.runtime.Permission(rs.runtime.Read), rs.runtime.Permission(rs.runtime.ReadUsers), withTx).Post("/export", rs.exportSnapshot)
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/{id}", rs.getRoom)
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/by-category", rs.getRoomsByCategory)
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/{id}/history", rs.GetRoomHistory)
		protected.With(rs.runtime.Permission(rs.runtime.Create), withTx).Post("/", rs.createRoom)
		protected.With(rs.runtime.Permission(rs.runtime.Update), withTx).Put("/{id}", rs.updateRoom)
		protected.With(rs.runtime.Permission(rs.runtime.Delete), withTx).Delete("/{id}", rs.deleteRoom)
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/buildings", rs.getBuildingList)
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/categories", rs.getCategoryList)
		protected.With(rs.runtime.Permission(rs.runtime.Read), withTx).Get("/available", rs.getAvailableRooms)
	})
	return router
}

type RoomRequest struct {
	Name     string  `json:"name"`
	Building string  `json:"building,omitempty"`
	Floor    *int    `json:"floor,omitempty"`
	Capacity *int    `json:"capacity,omitempty"`
	Category *string `json:"category,omitempty"`
	Color    *string `json:"color,omitempty"`
}

func (req *RoomRequest) Bind(_ *http.Request) error {
	if req.Capacity != nil && *req.Capacity <= 0 {
		return errors.New("capacity must be greater than zero")
	}
	return validation.ValidateStruct(req, validation.Field(&req.Name, validation.Required))
}

type RoomResponse struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Building        string    `json:"building,omitempty"`
	Floor           *int      `json:"floor,omitempty"`
	Capacity        *int      `json:"capacity,omitempty"`
	Category        *string   `json:"category,omitempty"`
	Color           *string   `json:"color,omitempty"`
	IsOccupied      bool      `json:"is_occupied"`
	GroupName       *string   `json:"group_name,omitempty"`
	CategoryName    *string   `json:"category_name,omitempty"`
	StudentCount    int       `json:"student_count"`
	SupervisorNames *string   `json:"supervisor_names,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func newRoomResponse(view RoomView) RoomResponse {
	return RoomResponse{
		ID: view.ID, Name: view.Name, Building: view.Building, Floor: view.Floor,
		Capacity: view.Capacity, Category: view.Category, Color: view.Color,
		IsOccupied: view.IsOccupied, GroupName: view.GroupName, CategoryName: view.CategoryName,
		StudentCount: view.StudentCount, SupervisorNames: view.SupervisorNames,
		CreatedAt: view.CreatedAt, UpdatedAt: view.UpdatedAt,
	}
}

func simpleRoomResponse(room facilities.Room) RoomResponse {
	return newRoomResponse(RoomView{Room: room})
}

func (rs *Resource) listRooms(w http.ResponseWriter, r *http.Request) {
	filter := facilities.RoomFilter{}
	if value := r.URL.Query().Get("building"); value != "" {
		filter.Building = &value
	}
	if value := r.URL.Query().Get("category"); value != "" {
		filter.Category = &value
	}
	rooms, err := rs.rooms.ListRooms(r.Context(), filter)
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	rooms = visibleRooms(rooms, r.URL.Query().Get("include_system") == "true")
	views, err := rs.runtime.Occupancy(r.Context(), rooms)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	responses := make([]RoomResponse, 0, len(views))
	for _, view := range views {
		responses = append(responses, newRoomResponse(view))
	}
	page, pageSize := rs.runtime.Pagination(r)
	rs.runtime.Paginated(w, r, http.StatusOK, responses, Pagination{Page: page, PageSize: pageSize, Total: len(responses)}, "Rooms retrieved successfully")
}

func (rs *Resource) getRoom(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid room ID"), "invalid_request")
		return
	}
	room, err := rs.rooms.FindRoom(r.Context(), id)
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	views, err := rs.runtime.Occupancy(r.Context(), []facilities.Room{room})
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	if len(views) != 1 {
		rs.runtime.Failure(w, r, FailureInternal, errors.New("room occupancy projection returned an invalid result"), "internal_error")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, newRoomResponse(views[0]), "Room retrieved successfully")
}

func (rs *Resource) createRoom(w http.ResponseWriter, r *http.Request) {
	req := new(RoomRequest)
	if err := render.Bind(r, req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err, "invalid_request")
		return
	}
	room, err := rs.rooms.CreateRoom(r.Context(), facilities.CreateRoom{
		Name: req.Name, Building: req.Building, Floor: req.Floor, Capacity: req.Capacity,
		Category: req.Category, Color: req.Color,
	})
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusCreated, simpleRoomResponse(room), "Room created successfully")
}

func (rs *Resource) updateRoom(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid room ID"), "invalid_request")
		return
	}
	req := new(RoomRequest)
	if err := render.Bind(r, req); err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, err, "invalid_request")
		return
	}
	room, err := rs.rooms.UpdateRoom(r.Context(), facilities.UpdateRoom{
		ID: id, Name: req.Name, Building: req.Building, Floor: req.Floor,
		Capacity: req.Capacity, Category: req.Category, Color: req.Color,
	})
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, simpleRoomResponse(room), "Room updated successfully")
}

func (rs *Resource) deleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid room ID"), "invalid_request")
		return
	}
	if err := rs.rooms.DeleteRoom(r.Context(), id); err != nil {
		if rs.runtime.ConstraintFailed(err) {
			rs.runtime.Failure(w, r, FailureConflict, errors.New("Raum kann nicht gelöscht werden: Raum wird noch von Gruppen verwendet"), "conflict") //nolint:staticcheck // stable user-facing contract
			return
		}
		rs.failure(w, r, err)
		return
	}
	rs.runtime.NoContent(w, r)
}

func (rs *Resource) getRoomsByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	if category == "" {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("category parameter is required"), "invalid_request")
		return
	}
	rooms, err := rs.rooms.ListRooms(r.Context(), facilities.RoomFilter{Category: &category})
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	responses := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		responses = append(responses, simpleRoomResponse(room))
	}
	rs.runtime.Success(w, r, http.StatusOK, responses, "Rooms retrieved successfully")
}

func (rs *Resource) getBuildingList(w http.ResponseWriter, r *http.Request) {
	rooms, err := rs.rooms.ListRooms(r.Context(), facilities.RoomFilter{})
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	buildings := uniqueRoomStrings(rooms, func(room facilities.Room) string { return room.Building })
	rs.runtime.Success(w, r, http.StatusOK, map[string][]string{"buildings": buildings}, "Building list retrieved successfully")
}

func (rs *Resource) getCategoryList(w http.ResponseWriter, r *http.Request) {
	rooms, err := rs.rooms.ListRooms(r.Context(), facilities.RoomFilter{})
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	categories := uniqueRoomStrings(rooms, func(room facilities.Room) string {
		if room.Category == nil {
			return ""
		}
		return *room.Category
	})
	rs.runtime.Success(w, r, http.StatusOK, map[string][]string{"categories": categories}, "Category list retrieved successfully")
}

func uniqueRoomStrings(rooms []facilities.Room, value func(facilities.Room) string) []string {
	seen := make(map[string]struct{}, len(rooms))
	for _, room := range rooms {
		if current := value(room); current != "" {
			seen[current] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for current := range seen {
		result = append(result, current)
	}
	slices.Sort(result)
	return result
}

func (rs *Resource) getAvailableRooms(w http.ResponseWriter, r *http.Request) {
	capacity := 0
	if value := r.URL.Query().Get("capacity"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			capacity = parsed
		}
	}
	rooms, err := rs.rooms.ListRooms(r.Context(), facilities.RoomFilter{MinimumCapacity: &capacity})
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	rooms = visibleRooms(rooms, r.URL.Query().Get("include_system") == "true")
	responses := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		responses = append(responses, simpleRoomResponse(room))
	}
	rs.runtime.Success(w, r, http.StatusOK, responses, "Available rooms retrieved successfully")
}

func visibleRooms(rooms []facilities.Room, includeSystem bool) []facilities.Room {
	if includeSystem {
		return rooms
	}
	visible := make([]facilities.Room, 0, len(rooms))
	for _, room := range rooms {
		if room.Name == facilities.WCRoomName || room.Name == facilities.WCRoomAliasName ||
			(room.IsSystem && room.Name != facilities.SchulhofRoomName) {
			continue
		}
		visible = append(visible, room)
	}
	return visible
}

func (rs *Resource) GetRoomHistory(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid room ID"), "invalid_request")
		return
	}
	ctx, enabled, capDays := rs.runtime.HistoryConfig(r.Context())
	r = r.WithContext(ctx)
	if !enabled {
		rs.runtime.Failure(w, r, FailureForbidden, errors.New("feature_disabled"), "forbidden")
		return
	}
	allowed, err := rs.runtime.HistoryAllowed(ctx)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	if !allowed {
		rs.runtime.Failure(w, r, FailureForbidden, errors.New("not_group_supervisor"), "forbidden")
		return
	}
	if capDays < 1 {
		capDays = 1
	}
	if capDays > 365 {
		capDays = 365
	}
	startOfToday, endOfToday := rs.runtime.TodayBounds()
	startTime := startOfToday.AddDate(0, 0, -(capDays - 1))
	endTime := endOfToday
	if value := r.URL.Query().Get("start"); value != "" {
		startTime, err = time.Parse(time.RFC3339, value)
		if err != nil {
			rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid start parameter, expected RFC3339"), "invalid_request")
			return
		}
	}
	if value := r.URL.Query().Get("end"); value != "" {
		endTime, err = time.Parse(time.RFC3339, value)
		if err != nil {
			rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid end parameter, expected RFC3339"), "invalid_request")
			return
		}
	}
	if startTime.After(endTime) {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("start must be before end"), "invalid_request")
		return
	}
	maxDuration := time.Duration(capDays) * 24 * time.Hour
	if endTime.Sub(startTime) > maxDuration {
		requestedStart := startTime
		startTime = endTime.Add(-maxDuration)
		rs.runtime.Log.Info("room history window clamped to retention cap",
			"room_id", id, "requested_start", requestedStart.Format(time.RFC3339),
			"clamped_start", startTime.Format(time.RFC3339), "end", endTime.Format(time.RFC3339),
			"cap_days", capDays)
	}
	history, err := rs.runtime.History(ctx, id, startTime, endTime)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInternal, err, "internal_error")
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, history, "Room history retrieved successfully")
}

func (rs *Resource) failure(w http.ResponseWriter, r *http.Request, err error) {
	kind, code := FailureInternal, facilities.ErrorCode(err)
	switch {
	case errors.Is(err, facilities.ErrInvalidRoom), errors.Is(err, facilities.ErrSystemRoomNameReserved),
		errors.Is(err, facilities.ErrRoomColorReserved):
		kind = FailureInvalid
	case errors.Is(err, facilities.ErrRoomNotFound):
		kind = FailureNotFound
	case errors.Is(err, facilities.ErrSystemRoomProtected):
		kind = FailureForbidden
	case errors.Is(err, facilities.ErrDuplicateRoom), errors.Is(err, facilities.ErrDuplicateToiletRoom),
		errors.Is(err, facilities.ErrRoomColorAlreadyInUse), errors.Is(err, facilities.ErrRoomInUse),
		errors.Is(err, facilities.ErrRoomRequiredByOffering):
		kind = FailureConflict
	}
	rs.runtime.Failure(w, r, kind, err, code)
}

func StatusOf(kind FailureKind) int {
	switch kind {
	case FailureInvalid:
		return http.StatusBadRequest
	case FailureForbidden:
		return http.StatusForbidden
	case FailureNotFound:
		return http.StatusNotFound
	case FailureConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
