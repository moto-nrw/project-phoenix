// backend/services/facilities/facilities_service.go
package facilities

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Operation name constants to avoid string duplication
const (
	opCreateRoom     = "create room"
	opUpdateRoom     = "update room"
	opFindToiletRoom = "find toilet room"
)

// service implements the facilities.Service interface
type service struct {
	roomRepo                         facilities.RoomRepository
	activeGroupRepo                  active.GroupRepository
	persons                          PersonQuery
	lockTemplateRecurrence           func(context.Context) error
	validateCareOfferingRoomDeletion func(context.Context, int64) error
}

// Person is the display projection of a supervising person.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
}

// PersonQuery resolves the supervising persons of a room by ID. The room
// repository only yields the person references (#2661); the names come
// from the People Directory through this port.
type PersonQuery interface {
	ListPersonsByID(context.Context, []int64) ([]Person, error)
}

// ServiceConfig carries the cross-domain guard needed by room deletion.
type ServiceConfig struct {
	RoomRepo        facilities.RoomRepository
	ActiveGroupRepo active.GroupRepository
	// PersonQuery resolves the supervisor names on the occupancy surfaces.
	// The service graph always wires it; a nil query (partial test wiring)
	// leaves SupervisorNames unset rather than failing every room read.
	PersonQuery                      PersonQuery
	LockTemplateRecurrence           func(context.Context) error
	ValidateCareOfferingRoomDeletion func(context.Context, int64) error
}

// wcRoomAliasNames lists the accepted canonical toilet-room aliases in
// canonical-first order. Only the exact-case names "WC" and "Toilette" are
// treated as the toilet system room — case variants like "wc" or "toilette"
// remain regular admin-managed rooms and are NOT auto-reused as the toilet
// special-room. This keeps the contract aligned across all layers
// (constants.IsWCRoomName, IsSystemRoomName, the rename/delete guards, the
// IoT scan-fallback switch in api/iot/checkin/workflow.go, and the partial
// unique index from migration 1.15.48 — all exact-case). Fixed array (not a
// slice) so a future caller can't append to a package global.
var wcRoomAliasNames = [...]string{constants.WCRoomName, constants.WCRoomAliasName}

// NewServiceWithConfig builds the facilities service with recurrence-aware
// room deletion validation.
func NewServiceWithConfig(cfg ServiceConfig) Service {
	return &service{
		roomRepo:                         cfg.RoomRepo,
		activeGroupRepo:                  cfg.ActiveGroupRepo,
		persons:                          cfg.PersonQuery,
		lockTemplateRecurrence:           cfg.LockTemplateRecurrence,
		validateCareOfferingRoomDeletion: cfg.ValidateCareOfferingRoomDeletion,
	}
}

// GetRoom retrieves a room by its ID
func (s *service) GetRoom(ctx context.Context, id int64) (*facilities.Room, error) {
	room, err := s.roomRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &FacilitiesError{Op: "get room", Err: ErrRoomNotFound}
	}
	return room, nil
}

// GetRoomWithOccupancy retrieves a room by its ID with occupancy status
func (s *service) GetRoomWithOccupancy(ctx context.Context, id int64) (RoomWithOccupancy, error) {
	result, err := s.roomRepo.FindWithOccupancy(ctx, id)
	if err != nil {
		// Only treat "no rows" as "room not found" - preserve other database errors
		if errors.Is(err, sql.ErrNoRows) {
			return RoomWithOccupancy{}, &FacilitiesError{Op: "get room with occupancy", Err: ErrRoomNotFound}
		}
		return RoomWithOccupancy{}, &FacilitiesError{Op: "get room with occupancy", Err: err}
	}
	if err := s.attachSupervisorNames(ctx, []facilities.RoomOccupancyRow{*result}, func(index int, names *string) { result.SupervisorNames = names }); err != nil {
		return RoomWithOccupancy{}, &FacilitiesError{Op: "get room with occupancy", Err: err}
	}

	// Convert result to RoomWithOccupancy
	return RoomWithOccupancy{
		Room: &facilities.Room{
			Model: base.Model{
				ID:        result.ID,
				CreatedAt: result.CreatedAt,
				UpdatedAt: result.UpdatedAt,
			},
			Name:     result.Name,
			Building: result.Building,
			Floor:    result.Floor,
			Capacity: result.Capacity,
			Category: result.Category,
			Color:    result.Color,
		},
		IsOccupied:      result.IsOccupied,
		GroupName:       result.GroupName,
		CategoryName:    result.CategoryName,
		StudentCount:    result.StudentCount,
		SupervisorNames: result.SupervisorNames,
	}, nil
}

// CreateRoom creates a new room
func (s *service) CreateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: translateValidationError(err)}
	}
	// Schulhof is an application invariant, not a normal room label. Reserve
	// every case variant because repository name lookup is case-insensitive;
	// only dedicated provisioning may create the exact canonical spelling.
	if strings.EqualFold(room.Name, constants.SchulhofRoomName) &&
		(room.Name != constants.SchulhofRoomName || !room.IsSystem) {
		return &FacilitiesError{Op: opCreateRoom, Err: ErrSystemRoomNameReserved}
	}

	// Set tenant ID from context
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		room.SetTenantID(tenantID)
	}

	if constants.IsWCRoomName(room.Name) {
		existingAlias, err := s.FindToiletRoom(ctx, 0)
		if err != nil && !errors.Is(err, ErrRoomNotFound) {
			return &FacilitiesError{Op: opCreateRoom, Err: err}
		}
		if existingAlias != nil {
			return &FacilitiesError{Op: opCreateRoom, Err: ErrDuplicateToiletRoom}
		}
	}

	// Check if a room with the same name already exists
	existing, err := s.roomRepo.FindByName(ctx, room.Name)
	if err == nil && existing != nil {
		return &FacilitiesError{Op: opCreateRoom, Err: ErrDuplicateRoom}
	}

	// Create the room
	if err := s.roomRepo.Create(ctx, room); err != nil {
		if isUniqueWCAliasViolation(err) {
			return &FacilitiesError{Op: opCreateRoom, Err: ErrDuplicateToiletRoom}
		}
		if isUniqueColorViolation(err) {
			return &FacilitiesError{Op: opCreateRoom, Err: ErrColorAlreadyInUse}
		}
		return &FacilitiesError{Op: opCreateRoom, Err: err}
	}

	return nil
}

// translateValidationError maps Validate() sentinels to the service-level
// errors that the API layer can render with the correct HTTP status. Any
// other validation error is forwarded unchanged so the existing
// ErrorInvalidRequest path renders 400.
func translateValidationError(err error) error {
	if errors.Is(err, facilities.ErrReservedColor) {
		return ErrColorReserved
	}
	return err
}

// isUniqueColorViolation reports whether err is a PostgreSQL 23505 raised by
// the partial unique index on facilities.rooms (tenant_id, lower(color)). The
// constraint name is checked so other future unique indexes on the table do
// not accidentally surface as "color already in use" toasts.
func isUniqueColorViolation(err error) bool {
	return base.IsUniqueViolationOn(err, facilities.RoomColorUniqueConstraintName)
}

// isUniqueWCAliasViolation reports whether err is a PostgreSQL 23505 raised
// by the partial unique index that enforces "at most one WC/Toilette alias
// per tenant". Hit only on the TOCTOU race the application-level guard in
// CreateRoom/UpdateRoom can't close — see migration 1.15.48.
func isUniqueWCAliasViolation(err error) bool {
	return base.IsUniqueViolationOn(err, facilities.RoomWCAliasUniqueConstraintName)
}

// equalStringPtr compares two *string for equality treating nil as "no value"
// — used to detect "color actually changed" without false-positives from the
// nil/empty distinction.
func equalStringPtr(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// FindToiletRoom iterates wcRoomAliasNames in canonical-first order and
// returns the first room whose name exactly matches one of the canonical
// aliases ("WC" or "Toilette"). Used by the auto-create flow and by the
// create/update guards that prevent a tenant from ending up with both
// canonical aliases at once. Returns ErrRoomNotFound (wrapped in
// FacilitiesError) when no canonical alias exists.
//
// Why the explicit IsWCRoomName re-check after FindByName: the repository
// matches case-insensitively via LOWER(name) = LOWER(?) — that CI behavior
// is required by the duplicate-name guard in CreateRoom and we don't want to
// change it. But the system-room contract is exact-case everywhere else
// (constants.IsWCRoomName, the partial unique index from migration 1.15.48,
// the IoT scan-fallback switch in api/iot/checkin/workflow.go). Without the
// re-check a lowercase "wc" room would be silently adopted as the toilet
// special-room here while remaining unprotected against rename/delete and
// invisible to the IoT scan path — exactly the split contract issue #1184
// review flagged. Skipping non-canonical rows keeps every layer aligned.
//
// Edge case: if a tenant somehow has both lowercase "wc" AND canonical "WC"
// (DB-level write that bypassed CreateRoom's CI duplicate guard), the
// FindByName CI lookup may return either row. If it returns the lowercase
// row we skip and try "Toilette" next; the canonical "WC" then goes
// undiscovered and the auto-create path will hit the CI duplicate guard.
// That stuck state requires DB cleanup but is not silent corruption.
func (s *service) FindToiletRoom(ctx context.Context, excludeRoomID int64) (*facilities.Room, error) {
	for _, roomName := range wcRoomAliasNames {
		room, err := s.roomRepo.FindByName(ctx, roomName)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, &FacilitiesError{Op: opFindToiletRoom, Err: err}
		}
		if !constants.IsWCRoomName(room.Name) {
			continue
		}
		if excludeRoomID > 0 && room.ID == excludeRoomID {
			continue
		}
		return room, nil
	}

	return nil, &FacilitiesError{Op: opFindToiletRoom, Err: ErrRoomNotFound}
}

// UpdateRoom updates an existing room
func (s *service) UpdateRoom(ctx context.Context, room *facilities.Room) error {
	// Validate room data
	if err := room.Validate(); err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: translateValidationError(err)}
	}

	// Check if room exists
	existingRoom, err := s.roomRepo.FindByID(ctx, room.ID)
	if err != nil {
		return &FacilitiesError{Op: opUpdateRoom, Err: ErrRoomNotFound}
	}

	// Block renaming system rooms (Schulhof, WC)
	if constants.IsSystemRoomName(existingRoom.Name) && room.Name != existingRoom.Name {
		return &FacilitiesError{Op: opUpdateRoom, Err: ErrSystemRoomProtected}
	}
	// Reserve Schulhof as a rename target as well. Without this guard, a
	// normal room could become Schulhof after timetable Start validated it,
	// leaving a generic active group in the dedicated supervision room.
	if existingRoom.Name != room.Name && strings.EqualFold(room.Name, constants.SchulhofRoomName) {
		return &FacilitiesError{Op: opUpdateRoom, Err: ErrSystemRoomNameReserved}
	}

	// Toilet-room color handling.
	//
	// The frontend strips the color picker for WC/Toilette, so a benign edit
	// (e.g. changing capacity) arrives with room.Color == nil regardless of
	// what's persisted. If we treated nil as "user wants to clear", every
	// non-color edit on a toilet room that still carries the legacy #4F46E5
	// bug-default would 403 — the migration covers most cases but not all
	// (e.g. a tenant that hasn't run migrations yet, or a system room
	// imported with a colour later).
	//
	// Strategy: treat a nil incoming colour as "request did not touch the
	// colour field" and silently preserve the existing value. Only block
	// when the request explicitly sets a different non-nil colour — that's
	// the path a malicious or buggy direct API call would take, and the one
	// the rule actually exists to stop.
	//
	// Side-effect of this strategy: a toilet room cannot have its colour
	// *cleared* via the API. If one somehow carries a stale colour (legacy
	// bug-default that escaped migration cleanup, or an imported dataset),
	// the only way to drop it is direct SQL. Acceptable trade-off — the WC
	// has no badge of its own, and the alternative is admins losing the
	// ability to edit any other field.
	//
	// Why both `room.Color != nil` AND equalStringPtr?
	//   - Inline `*room.Color != *existingRoom.Color` would NPE when the
	//     existing colour is nil and the request sets a new one (case A).
	//   - Dropping the outer guard would re-block benign updates whose
	//     incoming colour is nil but existing is non-nil (the very bug
	//     this comment block exists to fix).
	// Both checks together give: "block only when the request explicitly
	// names a different non-nil colour, ignore colour-field absence".
	//
	// The Schulhof is deliberately NOT covered here (#2405): schools
	// colour-code rooms and tablets and need the yard in that scheme, so its
	// colour follows the ordinary room rules — Validate()'s format and
	// reserved-hex checks plus the per-tenant uniqueness index. Rename and
	// delete protection above stays untouched.
	if constants.IsWCRoomName(existingRoom.Name) {
		if room.Color != nil && !equalStringPtr(room.Color, existingRoom.Color) {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrSystemRoomProtected}
		}
		room.Color = existingRoom.Color
	}

	// If name is changing, check for duplicates
	if existingRoom.Name != room.Name {
		if constants.IsWCRoomName(room.Name) {
			existingAlias, err := s.FindToiletRoom(ctx, room.ID)
			if err != nil && !errors.Is(err, ErrRoomNotFound) {
				return &FacilitiesError{Op: opUpdateRoom, Err: err}
			}
			if existingAlias != nil {
				return &FacilitiesError{Op: opUpdateRoom, Err: ErrDuplicateToiletRoom}
			}
		}

		existing, err := s.roomRepo.FindByName(ctx, room.Name)
		if err == nil && existing != nil && existing.ID != room.ID {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrDuplicateRoom}
		}
	}

	// Update the room
	if err := s.roomRepo.Update(ctx, room); err != nil {
		if isUniqueWCAliasViolation(err) {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrDuplicateToiletRoom}
		}
		if isUniqueColorViolation(err) {
			return &FacilitiesError{Op: opUpdateRoom, Err: ErrColorAlreadyInUse}
		}
		return &FacilitiesError{Op: opUpdateRoom, Err: err}
	}

	return nil
}

// DeleteRoom deletes a room by its ID
func (s *service) DeleteRoom(ctx context.Context, id int64) error {
	// Check if room exists
	existingRoom, err := s.roomRepo.FindByID(ctx, id)
	if err != nil {
		return &FacilitiesError{Op: "delete room", Err: ErrRoomNotFound}
	}

	// Block deletion of system rooms (Schulhof, WC)
	if constants.IsSystemRoomName(existingRoom.Name) {
		return &FacilitiesError{Op: "delete room", Err: ErrSystemRoomProtected}
	}

	// Best-effort pre-check: active groups would block deletion via FK RESTRICT.
	// The real protection is the DB constraint; this gives a user-friendly error message.
	activeGroups, preCheckErr := s.activeGroupRepo.FindActiveByRoomID(ctx, id)
	if preCheckErr != nil {
		slog.Warn("room_delete_precheck_failed",
			"room_id", id,
			"error", preCheckErr.Error(),
		)
	} else if len(activeGroups) > 0 {
		return &FacilitiesError{Op: "delete room", Err: ErrRoomInUse}
	}
	if err := s.validateRoomCareOfferingDeletion(ctx, id); err != nil {
		return err
	}

	// Delete the room
	if err := s.roomRepo.Delete(ctx, id); err != nil {
		return &FacilitiesError{Op: "delete room", Err: err}
	}

	return nil
}

func (s *service) validateRoomCareOfferingDeletion(ctx context.Context, id int64) error {
	if s.validateCareOfferingRoomDeletion == nil {
		return nil
	}
	if s.lockTemplateRecurrence == nil {
		return &FacilitiesError{Op: "delete room: lock timetable recurrence", Err: errors.New("template recurrence lock is not configured")}
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return &FacilitiesError{Op: "delete room: lock timetable recurrence", Err: err}
	}
	if err := s.validateCareOfferingRoomDeletion(ctx, id); err != nil {
		if errors.Is(err, enrollmentModels.ErrCareOfferingInvalid) {
			return &FacilitiesError{Op: "delete room", Err: ErrRoomRequiredByCareOffering}
		}
		return &FacilitiesError{Op: "delete room: validate care offerings", Err: err}
	}
	return nil
}

// ListRooms retrieves all rooms with occupancy status
func (s *service) ListRooms(ctx context.Context, options *base.QueryOptions) ([]RoomWithOccupancy, error) {
	results, err := s.roomRepo.ListWithOccupancy(ctx, options)
	if err != nil {
		// sql.ErrNoRows for list queries should return empty array, not error
		if errors.Is(err, sql.ErrNoRows) {
			return []RoomWithOccupancy{}, nil
		}
		return nil, &FacilitiesError{Op: "list rooms", Err: err}
	}
	if err := s.attachSupervisorNames(ctx, results, func(index int, names *string) { results[index].SupervisorNames = names }); err != nil {
		return nil, &FacilitiesError{Op: "list rooms", Err: err}
	}

	// Convert results to RoomWithOccupancy
	roomsWithOccupancy := make([]RoomWithOccupancy, len(results))
	for i, r := range results {
		roomsWithOccupancy[i] = RoomWithOccupancy{
			Room: &facilities.Room{
				Model: base.Model{
					ID:        r.ID,
					CreatedAt: r.CreatedAt,
					UpdatedAt: r.UpdatedAt,
				},
				Name:     r.Name,
				Building: r.Building,
				Floor:    r.Floor,
				Capacity: r.Capacity,
				Category: r.Category,
				Color:    r.Color,
			},
			IsOccupied:      r.IsOccupied,
			GroupName:       r.GroupName,
			CategoryName:    r.CategoryName,
			StudentCount:    r.StudentCount,
			SupervisorNames: r.SupervisorNames,
		}
	}

	return roomsWithOccupancy, nil
}

// FindRoomByName finds a room by its name
func (s *service) FindRoomByName(ctx context.Context, name string) (*facilities.Room, error) {
	room, err := s.roomRepo.FindByName(ctx, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &FacilitiesError{Op: "find room by name", Err: ErrRoomNotFound}
		}
		return nil, &FacilitiesError{Op: "find room by name", Err: err}
	}

	return room, nil
}

// FindRoomsByCategory finds rooms by category
func (s *service) FindRoomsByCategory(ctx context.Context, category string) ([]*facilities.Room, error) {
	rooms, err := s.roomRepo.FindByCategory(ctx, category)
	if err != nil {
		return nil, &FacilitiesError{Op: "find rooms by category", Err: err}
	}

	return rooms, nil
}

// GetAvailableRooms finds all rooms that can accommodate the given capacity
func (s *service) GetAvailableRooms(ctx context.Context, capacity int) ([]*facilities.Room, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get available rooms", Err: err}
	}

	// Filter rooms by capacity
	var availableRooms []*facilities.Room
	for _, room := range allRooms {
		if room.IsAvailable(capacity) {
			availableRooms = append(availableRooms, room)
		}
	}

	return availableRooms, nil
}

// GetAvailableRoomsWithOccupancy finds all rooms that can accommodate the given capacity
// and includes their current occupancy status
func (s *service) GetAvailableRoomsWithOccupancy(ctx context.Context, capacity int) ([]RoomWithOccupancy, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get available rooms with occupancy", Err: err}
	}

	// First pass: filter rooms by capacity and collect IDs
	var availableRooms []*facilities.Room
	var roomIDs []int64
	for _, room := range allRooms {
		if room.IsAvailable(capacity) {
			availableRooms = append(availableRooms, room)
			roomIDs = append(roomIDs, room.ID)
		}
	}

	// Batch fetch occupied room IDs (avoids N+1 query problem)
	occupiedRoomIDs, err := s.activeGroupRepo.GetOccupiedRoomIDs(ctx, roomIDs)
	if err != nil {
		return nil, &FacilitiesError{Op: "check room occupancy", Err: err}
	}

	// Build response with occupancy status from map lookup
	roomsWithOccupancy := make([]RoomWithOccupancy, 0, len(availableRooms))
	for _, room := range availableRooms {
		roomsWithOccupancy = append(roomsWithOccupancy, RoomWithOccupancy{
			Room:       room,
			IsOccupied: occupiedRoomIDs[room.ID],
		})
	}

	return roomsWithOccupancy, nil
}

// GetBuildingList returns a list of all buildings in the system
func (s *service) GetBuildingList(ctx context.Context) ([]string, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get building list", Err: err}
	}

	// Extract unique building names
	buildingMap := make(map[string]bool)
	for _, room := range allRooms {
		if room.Building != "" {
			buildingMap[room.Building] = true
		}
	}

	// Convert map to sorted slice
	buildings := slices.Sorted(maps.Keys(buildingMap))

	return buildings, nil
}

// GetCategoryList returns a list of all room categories in the system
func (s *service) GetCategoryList(ctx context.Context) ([]string, error) {
	// Get all rooms - using empty filter map for now
	allRooms, err := s.roomRepo.List(ctx, make(map[string]interface{}))
	if err != nil {
		return nil, &FacilitiesError{Op: "get category list", Err: err}
	}

	// Extract unique category names
	categoryMap := make(map[string]bool)
	for _, room := range allRooms {
		if room.Category != nil && *room.Category != "" {
			categoryMap[*room.Category] = true
		}
	}

	// Convert map to sorted slice
	categories := slices.Sorted(maps.Keys(categoryMap))

	return categories, nil
}

// GetRoomHistory retrieves the aggregated session timeline for a room.
// The handler is responsible for the GDPR feature gate and scope check;
// this method only verifies the room exists and delegates to the active
// group repository.
func (s *service) GetRoomHistory(ctx context.Context, roomID int64, startTime, endTime time.Time, supervisorStaffID *int64) ([]RoomSessionEntry, error) {
	if _, err := s.roomRepo.FindByID(ctx, roomID); err != nil {
		return nil, &FacilitiesError{Op: "get room history", Err: ErrRoomNotFound}
	}

	rows, err := s.activeGroupRepo.AggregateRoomSessions(ctx, roomID, startTime, endTime, supervisorStaffID)
	if err != nil {
		return nil, &FacilitiesError{Op: "get room history", Err: err}
	}

	entries := make([]RoomSessionEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, RoomSessionEntry{
			SessionID:       row.SessionID,
			StartedAt:       row.StartedAt,
			EndedAt:         row.EndedAt,
			DurationMinutes: row.DurationMinutes,
			ActivityName:    row.ActivityName,
			SupervisorName:  row.SupervisorName,
			StudentCount:    row.StudentCount,
		})
	}
	return entries, nil
}

// attachSupervisorNames resolves the supervising persons of every row and
// stores the distinct full names, alphabetically and comma separated, via
// set. Rows without a resolvable supervisor keep a nil value, like the
// previous SQL aggregate.
func (s *service) attachSupervisorNames(ctx context.Context, rows []facilities.RoomOccupancyRow, set func(int, *string)) error {
	if s.persons == nil {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.SupervisorPersonIDs...)
	}
	if len(ids) == 0 {
		return nil
	}
	persons, err := s.persons.ListPersonsByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]Person, len(persons))
	for _, person := range persons {
		byID[person.ID] = person
	}
	for index, row := range rows {
		names := make([]string, 0, len(row.SupervisorPersonIDs))
		for _, id := range row.SupervisorPersonIDs {
			if person, found := byID[id]; found {
				names = append(names, person.FirstName+" "+person.LastName)
			}
		}
		slices.Sort(names)
		names = slices.Compact(names)
		if len(names) == 0 {
			set(index, nil)
			continue
		}
		joined := strings.Join(names, ", ")
		set(index, &joined)
	}
	return nil
}
