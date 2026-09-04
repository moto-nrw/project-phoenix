// Package facilities is the public Facilities capability. It owns
// facilities.rooms and answers room lookups for every other owner without
// leaking the table into their repositories.
package facilities

import (
	"context"
	"errors"
	"strings"
	"time"
)

type roomNotFoundError struct{}

func (roomNotFoundError) Error() string       { return "room not found" }
func (roomNotFoundError) RepositoryNotFound() {}

var (
	ErrRoomNotFound                 error = roomNotFoundError{}
	ErrInvalidRoom                        = errors.New("invalid room")
	ErrDuplicateRoom                      = errors.New("Ein Raum mit diesem Namen existiert bereits")                                               //nolint:staticcheck // ST1005: stable user-facing contract
	ErrDuplicateToiletRoom                = errors.New("Es existiert bereits ein Toilettenraum (WC oder Toilette)")                                 //nolint:staticcheck // ST1005: stable user-facing contract
	ErrRoomColorReserved                  = errors.New("Diese Farbe ist für Statusbadges reserviert und kann nicht für Räume verwendet werden")     //nolint:staticcheck // ST1005: stable user-facing contract
	ErrRoomColorAlreadyInUse              = errors.New("Diese Farbe ist bereits einem anderen Raum zugeordnet")                                     //nolint:staticcheck // ST1005: stable user-facing contract
	ErrRoomInUse                          = errors.New("Raum kann nicht gelöscht werden: Raum wird aktuell von einer aktiven Gruppe verwendet")     //nolint:staticcheck // ST1005: stable user-facing contract
	ErrRoomRequiredByOffering             = errors.New("Raum kann nicht gelöscht werden: Raum wird für ein verknüpftes Betreuungsangebot benötigt") //nolint:staticcheck // ST1005: stable user-facing contract
	ErrRoomDeletionGuardUnavailable       = errors.New("room deletion guard is unavailable")
	ErrSystemRoomProtected                = errors.New("Systemraum kann nicht gelöscht oder umbenannt werden")      //nolint:staticcheck // ST1005: stable user-facing contract
	ErrSystemRoomNameReserved             = errors.New("Der Raumname „Schulhof“ ist für den Systemraum reserviert") //nolint:staticcheck // ST1005: stable user-facing contract
)

// Room is the facilities view of one physical room.
type Room struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `json:"name"`
	Building  string    `json:"building,omitempty"`
	Floor     *int      `json:"floor,omitempty"`
	Capacity  *int      `json:"capacity,omitempty"`
	Category  *string   `json:"category,omitempty"`
	Color     *string   `json:"color,omitempty"`
	IsSystem  bool      `json:"is_system"`
}

type Query interface {
	FindRoom(context.Context, int64) (Room, error)
	FindRoomForUpdate(context.Context, int64) (Room, error)
	FindRoomByName(context.Context, string) (Room, error)
	FindToiletRoom(context.Context, int64) (Room, error)
	ListRooms(context.Context, RoomFilter) ([]Room, error)
	ListRoomsPage(context.Context, RoomFilter, int, int) (RoomPage, error)
	ListRoomsByID(context.Context, []int64) ([]Room, error)
	LockRoomsByID(context.Context, []int64) ([]Room, error)
}

// Command owns room lifecycle writes.
type Command interface {
	CreateRoom(context.Context, CreateRoom) (Room, error)
	UpdateRoom(context.Context, UpdateRoom) (Room, error)
	DeleteRoom(context.Context, int64) error
}

type Capability interface {
	Query
	Command
}

type engine interface {
	RequireTenant(context.Context) error
	FindRoom(context.Context, int64) (Room, error)
	FindRoomForUpdate(context.Context, int64) (Room, error)
	FindRoomByName(context.Context, string) (Room, error)
	FindToiletRoom(context.Context, int64) (Room, error)
	ListRooms(context.Context, RoomFilter) ([]Room, error)
	ListRoomsPage(context.Context, RoomFilter, int, int) (RoomPage, error)
	ListRoomsByID(context.Context, []int64) ([]Room, error)
	LockRoomsByID(context.Context, []int64) ([]Room, error)
	CreateRoom(context.Context, CreateRoom) (Room, error)
	UpdateRoom(context.Context, UpdateRoom) (Room, error)
	DeleteRoom(context.Context, int64) error
	ObserveRejection(string, time.Duration, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("facilities: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) requireTenant(ctx context.Context, operation string) error {
	err := m.engine.RequireTenant(ctx)
	if err != nil {
		m.engine.ObserveRejection(operation, 0, err)
	}
	return err
}

func (m *Module) FindRoom(ctx context.Context, id int64) (Room, error) {
	if err := m.requireTenant(ctx, "find_room"); err != nil {
		return Room{}, err
	}
	if id <= 0 {
		err := ErrInvalidRoom
		m.engine.ObserveRejection("find_room", 0, err)
		return Room{}, err
	}
	return m.engine.FindRoom(ctx, id)
}

// FindRoomForUpdate returns a room while holding an update lock until the
// caller's transaction ends. Callers use it when a state check and a
// subsequent mutation must be atomic.
func (m *Module) FindRoomForUpdate(ctx context.Context, id int64) (Room, error) {
	if err := m.requireTenant(ctx, "find_room_for_update"); err != nil {
		return Room{}, err
	}
	if id <= 0 {
		err := ErrInvalidRoom
		m.engine.ObserveRejection("find_room_for_update", 0, err)
		return Room{}, err
	}
	return m.engine.FindRoomForUpdate(ctx, id)
}

func (m *Module) FindRoomByName(ctx context.Context, name string) (Room, error) {
	if err := m.requireTenant(ctx, "find_room_by_name"); err != nil {
		return Room{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		err := &InvalidRoomError{Reason: "room name is required"}
		m.engine.ObserveRejection("find_room_by_name", 0, err)
		return Room{}, err
	}
	return m.engine.FindRoomByName(ctx, name)
}

func (m *Module) FindToiletRoom(ctx context.Context, excludeRoomID int64) (Room, error) {
	if err := m.requireTenant(ctx, "find_toilet_room"); err != nil {
		return Room{}, err
	}
	if excludeRoomID < 0 {
		err := &InvalidRoomError{Reason: "excluded room ID cannot be negative"}
		m.engine.ObserveRejection("find_toilet_room", 0, err)
		return Room{}, err
	}
	return m.engine.FindToiletRoom(ctx, excludeRoomID)
}

func (m *Module) ListRooms(ctx context.Context, filter RoomFilter) ([]Room, error) {
	if err := m.requireTenant(ctx, "list_rooms"); err != nil {
		return nil, err
	}
	if filter.MinimumCapacity != nil && *filter.MinimumCapacity < 0 {
		err := &InvalidRoomError{Reason: "minimum capacity cannot be negative"}
		m.engine.ObserveRejection("list_rooms", 0, err)
		return nil, err
	}
	if filter.MaximumCapacity != nil && *filter.MaximumCapacity < 0 {
		err := &InvalidRoomError{Reason: "maximum capacity cannot be negative"}
		m.engine.ObserveRejection("list_rooms", 0, err)
		return nil, err
	}
	return m.engine.ListRooms(ctx, filter)
}

// ListRoomsPage returns one bounded room-list result and its total record
// count. Offset and limit are applied by the owner store.
func (m *Module) ListRoomsPage(ctx context.Context, filter RoomFilter, offset, limit int) (RoomPage, error) {
	if err := m.requireTenant(ctx, "list_rooms_page"); err != nil {
		return RoomPage{}, err
	}
	if offset < 0 || limit <= 0 {
		err := &InvalidRoomError{Reason: "room page is invalid"}
		m.engine.ObserveRejection("list_rooms_page", 0, err)
		return RoomPage{}, err
	}
	return m.engine.ListRoomsPage(ctx, filter, offset, limit)
}

// ListRoomsByID returns the rooms visible in the caller's transaction for
// the given IDs, sorted by name. Missing IDs are simply absent: consumers
// rendering a room name treat absence like the former LEFT JOIN.
func (m *Module) ListRoomsByID(ctx context.Context, ids []int64) ([]Room, error) {
	if err := m.requireTenant(ctx, "list_rooms_by_id"); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			err := ErrInvalidRoom
			m.engine.ObserveRejection("list_rooms_by_id", 0, err)
			return nil, err
		}
	}
	return m.engine.ListRoomsByID(ctx, ids)
}

// LockRoomsByID returns the visible rooms while retaining key-share locks for
// the caller's transaction. It is for restore flows that must keep a checked
// room reference valid until their dependent INSERT finishes.
func (m *Module) LockRoomsByID(ctx context.Context, ids []int64) ([]Room, error) {
	if err := m.requireTenant(ctx, "lock_rooms_by_id"); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			err := ErrInvalidRoom
			m.engine.ObserveRejection("lock_rooms_by_id", 0, err)
			return nil, err
		}
	}
	return m.engine.LockRoomsByID(ctx, ids)
}

func (m *Module) CreateRoom(ctx context.Context, input CreateRoom) (Room, error) {
	started := time.Now()
	if err := m.requireTenant(ctx, "create_room"); err != nil {
		return Room{}, err
	}
	if err := normalizeAndValidateCreateRoom(&input); err != nil {
		m.engine.ObserveRejection("create_room", time.Since(started), err)
		return Room{}, err
	}
	if strings.EqualFold(input.Name, SchulhofRoomName) && (input.Name != SchulhofRoomName || !input.IsSystem) {
		err := ErrSystemRoomNameReserved
		m.engine.ObserveRejection("create_room", time.Since(started), err)
		return Room{}, err
	}
	return m.engine.CreateRoom(ctx, input)
}

func (m *Module) UpdateRoom(ctx context.Context, input UpdateRoom) (Room, error) {
	started := time.Now()
	if err := m.requireTenant(ctx, "update_room"); err != nil {
		return Room{}, err
	}
	if input.ID <= 0 {
		err := &InvalidRoomError{Reason: "room ID is required"}
		m.engine.ObserveRejection("update_room", time.Since(started), err)
		return Room{}, err
	}
	if err := normalizeAndValidateRoom(&input.Name, input.Capacity, &input.Color); err != nil {
		m.engine.ObserveRejection("update_room", time.Since(started), err)
		return Room{}, err
	}
	return m.engine.UpdateRoom(ctx, input)
}

func (m *Module) DeleteRoom(ctx context.Context, id int64) error {
	if err := m.requireTenant(ctx, "delete_room"); err != nil {
		return err
	}
	if id <= 0 {
		err := &InvalidRoomError{Reason: "room ID is required"}
		m.engine.ObserveRejection("delete_room", 0, err)
		return err
	}
	return m.engine.DeleteRoom(ctx, id)
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrRoomNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidRoom):
		return "invalid"
	case errors.Is(err, ErrDuplicateRoom):
		return "duplicate"
	case errors.Is(err, ErrDuplicateToiletRoom):
		return "duplicate_toilet_room"
	case errors.Is(err, ErrRoomColorReserved):
		return "color_reserved"
	case errors.Is(err, ErrRoomColorAlreadyInUse):
		return "color_already_in_use"
	case errors.Is(err, ErrRoomInUse):
		return "room_in_use"
	case errors.Is(err, ErrRoomRequiredByOffering):
		return "room_required_by_offering"
	case errors.Is(err, ErrRoomDeletionGuardUnavailable):
		return "deletion_guard_unavailable"
	case errors.Is(err, ErrSystemRoomNameReserved):
		return "system_name_reserved"
	case errors.Is(err, ErrSystemRoomProtected):
		return "system_room_protected"
	default:
		return "internal_error"
	}
}
