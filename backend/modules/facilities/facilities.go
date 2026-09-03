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
	FindRoomByName(context.Context, string) (Room, error)
	FindToiletRoom(context.Context, int64) (Room, error)
	ListRooms(context.Context, RoomFilter) ([]Room, error)
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
	FindRoom(context.Context, int64) (Room, error)
	FindRoomByName(context.Context, string) (Room, error)
	FindToiletRoom(context.Context, int64) (Room, error)
	ListRooms(context.Context, RoomFilter) ([]Room, error)
	ListRoomsByID(context.Context, []int64) ([]Room, error)
	LockRoomsByID(context.Context, []int64) ([]Room, error)
	CreateRoom(context.Context, CreateRoom) (Room, error)
	UpdateRoom(context.Context, UpdateRoom) (Room, error)
	DeleteRoom(context.Context, int64) error
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("facilities: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindRoom(ctx context.Context, id int64) (Room, error) {
	if id <= 0 {
		return Room{}, ErrInvalidRoom
	}
	return m.engine.FindRoom(ctx, id)
}

func (m *Module) FindRoomByName(ctx context.Context, name string) (Room, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Room{}, &InvalidRoomError{Reason: "room name is required"}
	}
	return m.engine.FindRoomByName(ctx, name)
}

func (m *Module) FindToiletRoom(ctx context.Context, excludeRoomID int64) (Room, error) {
	if excludeRoomID < 0 {
		return Room{}, &InvalidRoomError{Reason: "excluded room ID cannot be negative"}
	}
	return m.engine.FindToiletRoom(ctx, excludeRoomID)
}

func (m *Module) ListRooms(ctx context.Context, filter RoomFilter) ([]Room, error) {
	if filter.MinimumCapacity != nil && *filter.MinimumCapacity < 0 {
		return nil, &InvalidRoomError{Reason: "minimum capacity cannot be negative"}
	}
	if filter.MaximumCapacity != nil && *filter.MaximumCapacity < 0 {
		return nil, &InvalidRoomError{Reason: "maximum capacity cannot be negative"}
	}
	return m.engine.ListRooms(ctx, filter)
}

// ListRoomsByID returns the rooms visible in the caller's transaction for
// the given IDs, sorted by name. Missing IDs are simply absent: consumers
// rendering a room name treat absence like the former LEFT JOIN.
func (m *Module) ListRoomsByID(ctx context.Context, ids []int64) ([]Room, error) {
	if len(ids) == 0 {
		return []Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidRoom
		}
	}
	return m.engine.ListRoomsByID(ctx, ids)
}

// LockRoomsByID returns the visible rooms while retaining key-share locks for
// the caller's transaction. It is for restore flows that must keep a checked
// room reference valid until their dependent INSERT finishes.
func (m *Module) LockRoomsByID(ctx context.Context, ids []int64) ([]Room, error) {
	if len(ids) == 0 {
		return []Room{}, nil
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvalidRoom
		}
	}
	return m.engine.LockRoomsByID(ctx, ids)
}

func (m *Module) CreateRoom(ctx context.Context, input CreateRoom) (Room, error) {
	if err := normalizeAndValidateCreateRoom(&input); err != nil {
		return Room{}, err
	}
	if strings.EqualFold(input.Name, SchulhofRoomName) && (input.Name != SchulhofRoomName || !input.IsSystem) {
		return Room{}, ErrSystemRoomNameReserved
	}
	return m.engine.CreateRoom(ctx, input)
}

func (m *Module) UpdateRoom(ctx context.Context, input UpdateRoom) (Room, error) {
	if input.ID <= 0 {
		return Room{}, &InvalidRoomError{Reason: "room ID is required"}
	}
	if err := normalizeAndValidateRoom(&input.Name, input.Capacity, &input.Color); err != nil {
		return Room{}, err
	}
	return m.engine.UpdateRoom(ctx, input)
}

func (m *Module) DeleteRoom(ctx context.Context, id int64) error {
	if id <= 0 {
		return &InvalidRoomError{Reason: "room ID is required"}
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
