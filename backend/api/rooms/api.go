package rooms

import (
	"context"
	"errors"
	"log/slog"
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
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilityService "github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource defines the rooms API resource. ActiveService, PersonService,
// EducationService, and ListExportService are wired post-construction in
// api/base.go.
type Resource struct {
	ResourceConfig
	ActiveService     activeService.Service
	PersonService     userService.PersonService
	EducationService  educationService.Service
	ListExportService *listexport.RendererService
}

// ResourceConfig wires the rooms resource's collaborators. Kept as a struct
// so future additions don't churn every call site (the resource needs
// settings + user context for the GDPR-gated room history endpoint).
type ResourceConfig struct {
	FacilityService    facilityService.Service
	SettingsService    configService.SettingsService
	UserContextService userContextService.UserContextService
	Logger             *slog.Logger
	DB                 *bun.DB
}

// NewResource creates a new rooms resource. Required collaborators are
// nil-checked at construction so a wiring typo in initializeAPIResources
// surfaces at boot, not as a silent 403 for every tenant (nil
// SettingsService would otherwise fall through to the registry defaults
// and short-circuit the room-history endpoint to feature_disabled) or as a
// per-request panic (nil FacilityService / UserContextService). Logger is
// optional — handler paths fall back to slog.Default() via
// roomHistoryLogger.
func NewResource(cfg ResourceConfig) *Resource {
	if cfg.FacilityService == nil {
		panic("rooms.NewResource: FacilityService is required")
	}
	if cfg.SettingsService == nil {
		panic("rooms.NewResource: SettingsService is required")
	}
	if cfg.UserContextService == nil {
		panic("rooms.NewResource: UserContextService is required")
	}
	if cfg.DB == nil {
		panic("rooms.NewResource: DB is required")
	}
	return &Resource{ResourceConfig: cfg}
}

// Router returns a configured router for room endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {

		// Read operations require rooms:read permission
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/", rs.listRooms)
		r.With(authorize.RequiresAllPermissions(permissions.RoomsRead, permissions.UsersRead), withTx).Post("/export", rs.exportSnapshot)
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/{id}", rs.getRoom)
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/by-category", rs.getRoomsByCategory)
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/{id}/history", rs.GetRoomHistory)

		// Write operations require specific permissions
		r.With(authorize.RequiresPermission(permissions.RoomsCreate), withTx).Post("/", rs.createRoom)
		r.With(authorize.RequiresPermission(permissions.RoomsUpdate), withTx).Put("/{id}", rs.updateRoom)
		r.With(authorize.RequiresPermission(permissions.RoomsDelete), withTx).Delete("/{id}", rs.deleteRoom)

		// Advanced operations
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/buildings", rs.getBuildingList)
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/categories", rs.getCategoryList)
		r.With(authorize.RequiresPermission(permissions.RoomsRead), withTx).Get("/available", rs.getAvailableRooms)
	})

	return r
}

// RoomRequest represents a room request payload
type RoomRequest struct {
	Name     string  `json:"name"`
	Building string  `json:"building,omitempty"`
	Floor    *int    `json:"floor,omitempty"`
	Capacity *int    `json:"capacity,omitempty"`
	Category *string `json:"category,omitempty"`
	Color    *string `json:"color,omitempty"`
}

// Bind validates the room request
func (req *RoomRequest) Bind(_ *http.Request) error {
	if req.Capacity != nil && *req.Capacity <= 0 {
		return facilities.ErrCapacityNotPositive
	}
	return validation.ValidateStruct(req, validation.Field(&req.Name, validation.Required))
}

// RoomResponse represents a room response
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

// Convert a RoomWithOccupancy to a RoomResponse
func newRoomResponse(roomWithOcc facilityService.RoomWithOccupancy) RoomResponse {
	return RoomResponse{
		ID:              roomWithOcc.ID,
		Name:            roomWithOcc.Name,
		Building:        roomWithOcc.Building,
		Floor:           roomWithOcc.Floor,
		Capacity:        roomWithOcc.Capacity,
		Category:        roomWithOcc.Category,
		Color:           roomWithOcc.Color,
		IsOccupied:      roomWithOcc.IsOccupied,
		GroupName:       roomWithOcc.GroupName,
		CategoryName:    roomWithOcc.CategoryName,
		StudentCount:    roomWithOcc.StudentCount,
		SupervisorNames: roomWithOcc.SupervisorNames,
		CreatedAt:       roomWithOcc.CreatedAt,
		UpdatedAt:       roomWithOcc.UpdatedAt,
	}
}

// Convert a Room to a RoomResponse (without occupancy info)
func newRoomResponseSimple(room *facilities.Room) RoomResponse {
	return newRoomResponse(facilityService.RoomWithOccupancy{
		Room:         room,
		IsOccupied:   false,
		GroupName:    nil,
		CategoryName: nil,
	})
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

	// Keep Schulhof available for staff planning while hiding infrastructure
	// rooms such as WC. The visibility expression stays inside one grouped AND
	// so it cannot bypass tenant, building, or category predicates.
	if r.URL.Query().Get("include_system") != "true" {
		staffVisible := base.NewFilter().
			Equal("is_system", false).
			NotIn("name", constants.WCRoomName, constants.WCRoomAliasName)
		staffVisible.Or(*base.NewFilter().Equal("name", constants.SchulhofRoomName))
		queryOptions.Filter.And(*staffVisible)
	}

	// Add pagination if provided
	page, pageSize := common.ParsePagination(r)
	queryOptions.WithPagination(page, pageSize)

	// Get rooms with occupancy from service
	roomsWithOccupancy, err := rs.FacilityService.ListRooms(r.Context(), queryOptions)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	roomResponses := make([]RoomResponse, len(roomsWithOccupancy))
	for i, roomWithOcc := range roomsWithOccupancy {
		roomResponses[i] = newRoomResponse(roomWithOcc)
	}

	// Use common paginated response
	common.RespondPaginated(w, r, http.StatusOK, roomResponses, common.PaginationParams{Page: page, PageSize: pageSize, Total: len(roomsWithOccupancy)}, "Rooms retrieved successfully")
}

// getRoom handles getting a room by ID
func (rs *Resource) getRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidRoomID)))
		return
	}

	// Get room with occupancy from service
	roomWithOcc, err := rs.FacilityService.GetRoomWithOccupancy(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("room not found")))
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, newRoomResponse(roomWithOcc), "Room retrieved successfully")
}

// createRoom handles creating a new room
func (rs *Resource) createRoom(w http.ResponseWriter, r *http.Request) {
	req := &RoomRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
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

	// Validation runs inside CreateRoom. Keeping it there as the single
	// source of truth lets the service translate facilities.ErrReservedColor
	// to the German service-level error which ErrorRenderer maps to 400.

	// Create room using service
	tenantID := tenant.FromContext(r.Context())
	err := tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.FacilityService.CreateRoom(ctx, room)
	})
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Return response
	common.Respond(w, r, http.StatusCreated, newRoomResponseSimple(room), "Room created successfully")
}

// updateRoom handles updating an existing room
func (rs *Resource) updateRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidRoomID)))
		return
	}

	// Get existing room
	room, err := rs.FacilityService.GetRoom(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("room not found")))
		return
	}

	// Parse request
	req := &RoomRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Update room fields
	room.Name = req.Name
	room.Building = req.Building
	room.Floor = req.Floor
	room.Capacity = req.Capacity
	room.Category = req.Category
	room.Color = req.Color

	// Validation runs inside UpdateRoom; ErrorRenderer translates German
	// ErrColorReserved / ErrColorAlreadyInUse to 400 / 409 respectively.

	// Update room using service
	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.FacilityService.UpdateRoom(ctx, room)
	})
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, newRoomResponseSimple(room), "Room updated successfully")
}

// deleteRoom handles deleting a room
func (rs *Resource) deleteRoom(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidRoomID)))
		return
	}

	// Delete room using service
	tenantID := tenant.FromContext(r.Context())
	err = tenant.WithTenantTx(r.Context(), rs.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return rs.FacilityService.DeleteRoom(ctx, id)
	})
	if err != nil {
		// Catch constraint violations not covered by service pre-checks
		// (e.g., composite FK SET NULL causing NOT NULL violation on tenant_id)
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Raum kann nicht gelöscht werden: Raum wird noch von Gruppen verwendet"))
			return
		}
		common.RenderError(w, r, ErrorRenderer(err))
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
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("category parameter is required")))
		return
	}

	// Get rooms by category
	rooms, err := rs.FacilityService.FindRoomsByCategory(r.Context(), category)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Convert to response
	roomResponses := make([]RoomResponse, len(rooms))
	for i, room := range rooms {
		roomResponses[i] = newRoomResponseSimple(room)
	}

	// Return response
	common.Respond(w, r, http.StatusOK, roomResponses, "Rooms retrieved successfully")
}

// getBuildingList handles getting a list of building names
func (rs *Resource) getBuildingList(w http.ResponseWriter, r *http.Request) {
	// Get buildings list
	buildings, err := rs.FacilityService.GetBuildingList(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Keep Schulhof available for staff planning while hiding other system
	// rooms. Filtered here rather than in GetAvailableRooms because the IoT
	// kiosk endpoint shares that service method and must keep seeing them.
	includeSystem := r.URL.Query().Get("include_system") == "true"

	// Convert to response
	roomResponses := make([]RoomResponse, 0, len(rooms))
	for _, room := range rooms {
		if !includeSystem && (constants.IsWCRoomName(room.Name) ||
			(room.IsSystem && room.Name != constants.SchulhofRoomName)) {
			continue
		}
		roomResponses = append(roomResponses, newRoomResponseSimple(room))
	}

	// Return response
	common.Respond(w, r, http.StatusOK, roomResponses, "Available rooms retrieved successfully")
}

// GetRoomHistory returns the aggregated session timeline for a room.
// Exported because the rooms_test package binds it directly into a test
// router (see Rule 5 in backend-conventions.md — no separate
// `GetRoomHistoryHandler()` wrapper).
//
// Privacy: this endpoint is gated by gdpr.attendance_log_enabled, scoped by
// gdpr.attendance_log_scope (admin / supervisor-only / all_staff), and the
// requested time range is capped by gdpr.room_detail_visible_days. No
// per-student IDs or names leave the backend — per-child movement detail
// lives behind /students/{id}/attendance-history under its own gates.
func (rs *Resource) GetRoomHistory(w http.ResponseWriter, r *http.Request) {
	// One batch query for the three gates below; the scope check in
	// resolveRoomHistorySupervisorFilter reads from the snapshot too
	// (issue #2065).
	ctx := common.PrefetchSettings(r.Context(), rs.SettingsService,
		configModel.KeyAttendanceLogEnabled,
		configModel.KeyRoomDetailVisibleDays,
		configModel.KeyAttendanceLogScope,
	)
	r = r.WithContext(ctx)
	logger := rs.roomHistoryLogger()

	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidRoomID)))
		return
	}

	// 1. Feature gate
	if !configService.ResolveBoolOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogEnabled, false, logger) {
		// The "feature_disabled" literal is part of the cross-boundary
		// contract with the room drawer: the Next.js proxy at
		// /api/rooms/[id]/history matches this exact string against the
		// 403 body and translates it into a non-error signal so the
		// drawer hides the Belegungshistorie section deliberately.
		// Mirror in frontend: ROOM_HISTORY_STATUS_FEATURE_DISABLED in
		// frontend/src/lib/room-helpers.ts — keep in sync.
		common.RenderError(w, r, common.ErrorForbidden(errors.New("feature_disabled")))
		return
	}

	// 2. Scope check — see resolveRoomHistorySupervisorFilter. If handled
	// is true the helper already wrote the response (403 / 500) and we
	// must stop.
	supervisorStaffID, handled := rs.resolveRoomHistorySupervisorFilter(w, r, id, logger)
	if handled {
		return
	}

	// 3. Resolve range cap. The setting registry already constrains the
	// value (Min=1, Max=365), but a defensive clamp here protects against
	// hand-edited DB rows that bypassed the registry validator.
	const roomCapHardMax = 365
	roomCap := configService.ResolveIntOrDefault(ctx, rs.SettingsService, configModel.KeyRoomDetailVisibleDays, 7, logger)
	if roomCap < 1 {
		roomCap = 1
	}
	if roomCap > roomCapHardMax {
		roomCap = roomCapHardMax
	}

	// 4. Parse range and clamp
	today := timezone.Today()
	endOfToday := timezone.EndOfDay(today)
	defaultStart := today.AddDate(0, 0, -(roomCap - 1))

	startTime := defaultStart
	endTime := endOfToday

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		parsedStart, parseErr := time.Parse(time.RFC3339, startStr)
		if parseErr != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start parameter, expected RFC3339")))
			return
		}
		startTime = parsedStart
	}

	if endStr := r.URL.Query().Get("end"); endStr != "" {
		parsedEnd, parseErr := time.Parse(time.RFC3339, endStr)
		if parseErr != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end parameter, expected RFC3339")))
			return
		}
		endTime = parsedEnd
	}

	if startTime.After(endTime) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("start must be before end")))
		return
	}

	// Silently narrow the window when the caller asks for more than the
	// tenant's room_detail_visible_days cap allows. The clamp is
	// intentional — the cap is a GDPR retention boundary, not a hint — but
	// log it at Info so anyone debugging an "empty results" report has a
	// breadcrumb without needing to repro.
	maxDuration := time.Duration(roomCap) * 24 * time.Hour
	if endTime.Sub(startTime) > maxDuration {
		requestedStart := startTime
		startTime = endTime.Add(-maxDuration)
		logger.Info("room history window clamped to retention cap",
			"room_id", id,
			"requested_start", requestedStart.Format(time.RFC3339),
			"clamped_start", startTime.Format(time.RFC3339),
			"end", endTime.Format(time.RFC3339),
			"cap_days", roomCap,
		)
	}

	history, err := rs.FacilityService.GetRoomHistory(ctx, id, startTime, endTime, supervisorStaffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Return response
	common.Respond(w, r, http.StatusOK, history, "Room history retrieved successfully")
}

// resolveRoomHistorySupervisorFilter applies the gdpr.attendance_log_scope
// rule. Admin callers see every session without a staff lookup. Non-admin
// callers must resolve to a staff row before the tenant scope is applied:
// all_staff sees every session, while group_supervisors_only is filtered to
// sessions they supervise. The "scope != all_staff" condition is inverted on
// purpose: an unknown / typo'd setting value falls through to the safe
// supervisor-only path instead of silently disabling the filter.
//
// Return contract:
//
//	(filter, false) — caller continues; filter may be nil (admin/all_staff)
//	(nil,    true)  — helper already wrote a 403/500 response; caller MUST return
//
// Distinguishing ErrUserNotLinkedToStaff (legitimate 403) from any other
// error is deliberate: a flaky users.staff lookup would otherwise silently
// surface as "forbidden" with no log entry.
func (rs *Resource) resolveRoomHistorySupervisorFilter(
	w http.ResponseWriter,
	r *http.Request,
	roomID int64,
	logger *slog.Logger,
) (*int64, bool) {
	ctx := r.Context()
	if authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx)) {
		return nil, false
	}

	staff, err := rs.UserContextService.GetCurrentStaff(ctx)
	switch {
	case errors.Is(err, userContextService.ErrUserNotLinkedToStaff):
		common.RenderError(w, r, common.ErrorForbidden(errors.New("not_group_supervisor")))
		return nil, true
	case err != nil:
		logger.Warn("get current staff failed for room history",
			"room_id", roomID,
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return nil, true
	case staff == nil:
		// Defensive: service contract is (non-nil, nil) or (nil, err);
		// this branch only triggers if that contract gets broken.
		logger.Warn("get current staff returned nil staff with nil error",
			"room_id", roomID,
		)
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("unexpected nil staff")))
		return nil, true
	}

	scope := configService.ResolveStringOrDefault(ctx, rs.SettingsService, configModel.KeyAttendanceLogScope, configModel.AttendanceLogScopeGroupSupervisorsOnly, logger)
	if scope == configModel.AttendanceLogScopeAllStaff {
		return nil, false
	}

	staffID := staff.ID
	return &staffID, false
}

// roomHistoryLogger returns a scoped logger, falling back to slog.Default
// so missing wiring never crashes the handler. Uses a distinct "endpoint"
// key for the inner scope; the resource logger already binds
// handler=rooms, so reusing "handler" here would emit two values for the
// same key.
func (rs *Resource) roomHistoryLogger() *slog.Logger {
	if rs.Logger != nil {
		return rs.Logger.With("endpoint", "room_history")
	}
	return slog.Default().With("endpoint", "room_history")
}
