// Package facilities is the public Facilities capability. It owns
// facilities.rooms and answers room lookups for every other owner without
// leaking the table into their repositories.
package facilities

import (
	"context"
	"errors"
	"time"
)

type roomNotFoundError struct{}

func (roomNotFoundError) Error() string       { return "room not found" }
func (roomNotFoundError) RepositoryNotFound() {}

var (
	ErrRoomNotFound                 error = roomNotFoundError{}
	ErrInvalidRoom                        = errors.New("invalid room")
	ErrDuplicateRoom                      = errors.New("ein Raum mit diesem Namen existiert bereits")
	ErrDuplicateToiletRoom                = errors.New("es existiert bereits ein Toilettenraum (WC oder Toilette)")
	ErrRoomColorReserved                  = errors.New("diese Farbe ist für Statusbadges reserviert und kann nicht für Räume verwendet werden")
	ErrRoomInUse                          = errors.New("raum kann nicht gelöscht werden: Raum wird aktuell von einer aktiven Gruppe verwendet")
	ErrRoomRequiredByOffering             = errors.New("raum kann nicht gelöscht werden: Raum wird für ein verknüpftes Betreuungsangebot benötigt")
	ErrRoomDeletionGuardUnavailable       = errors.New("room deletion guard is unavailable")
	ErrSystemRoomProtected                = errors.New("systemraum kann nicht gelöscht oder umbenannt werden")
	ErrSystemRoomNameReserved             = errors.New("der Raumname „Schulhof“ ist für den Systemraum reserviert")
	// ErrToiletRoomNotReleasable rejects releasing a toilet room. The toilets
	// are infrastructure the kiosk drives through its own short-stay flow; a
	// released WC would offer children a permanent destination the product
	// never intended and that no school asked for.
	ErrToiletRoomNotReleasable = errors.New("toilettenräume können nicht als offener Raum freigegeben werden")
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
	// Color is the room's own color, set by the school as an override. It is
	// what the room form edits; badges read BadgeColor.
	Color *string `json:"color,omitempty"`
	// CategoryColor is the catalog color of the room's category, nil when the
	// category is not in the catalog. Only reads fill it.
	CategoryColor *string `json:"category_color,omitempty"`
	IsSystem      bool    `json:"is_system"`
	// IsOpenRoom reports whether the OGS administration has permanently
	// released this room for use ("offener Raum", #3062). The release says
	// nothing about who supervises the room and grants no home-checkout
	// rights — those stay separate decisions.
	IsOpenRoom bool `json:"is_open_room"`
}

// BadgeColor is the color a presence badge shows for the room (#4352): the
// room's own color first, otherwise its category's. System rooms take no
// category color: the Schulhof keeps its orange status default and the
// toilet rooms have no badge of their own. Nil leaves the badge to the
// frontend default.
func (r Room) BadgeColor() *string {
	if r.Color != nil && *r.Color != "" {
		return r.Color
	}
	if r.IsSystem {
		return nil
	}
	return r.CategoryColor
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

// Catalog is every Facilities operation except the room deletion: the room
// reads and the create and update writes. The composition exposes it before the owners
// the deletion guard asks exist.
type Catalog interface {
	Query
	CreateRoom(context.Context, CreateRoom) (Room, error)
	UpdateRoom(context.Context, UpdateRoom) (Room, error)
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
	case errors.Is(err, ErrRoomCategoryNotFound):
		return "category_not_found"
	case errors.Is(err, ErrDuplicateRoomCategory):
		return "duplicate_category"
	case errors.Is(err, ErrRoomCategoryInUse):
		return "category_in_use"
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
	case errors.Is(err, ErrToiletRoomNotReleasable):
		return "toilet_room_not_releasable"
	default:
		return "internal_error"
	}
}
