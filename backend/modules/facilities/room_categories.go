package facilities

import (
	"context"
	"errors"
	"time"
)

// RoomCategory is one entry of the school's room category catalog (#4352).
// Rooms name their category by text; the catalog gives every category one
// badge color, so rooms of the same kind look alike in the presence badges.
type RoomCategory struct {
	ID        int64
	Name      string
	Color     string
	RoomCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RoomCategoryInput is the caller-controlled part of a category write.
type RoomCategoryInput struct {
	Name  string
	Color string
}

// RoomCategories owns the room category catalog of the caller's school.
type RoomCategories interface {
	ListRoomCategories(context.Context) ([]RoomCategory, error)
	CreateRoomCategory(context.Context, RoomCategoryInput) (RoomCategory, error)
	UpdateRoomCategory(context.Context, int64, RoomCategoryInput) (RoomCategory, error)
	DeleteRoomCategory(context.Context, int64) error
	// SeedDefaultRoomCategories creates DefaultRoomCategories at a new
	// school; a category the school already has is kept as it is.
	SeedDefaultRoomCategories(context.Context) error
}

// MaxRoomCategoryNameLength bounds a category name; the frontend form
// mirrors it.
const MaxRoomCategoryNameLength = 60

// DefaultRoomCategories are the categories a new school starts with: the four
// the room form offered as fixed options before the catalog existed. The
// colors sit outside the status-badge palette (reservedRoomColors) and are
// muted on purpose, so the status colors keep standing out. Keep them in sync
// with migration 1.15.472, which backfills existing schools.
var DefaultRoomCategories = []RoomCategoryInput{
	{Name: "Gruppenraum", Color: "#283590"},
	{Name: "Normaler Raum", Color: "#374750"},
	{Name: "Themenraum", Color: "#6A1B90"},
	{Name: "Sport", Color: "#827710"},
}

var (
	ErrRoomCategoryNotFound  = errors.New("room category not found")
	ErrDuplicateRoomCategory = errors.New("a room category with this name already exists")
	// ErrRoomCategoryInUse keeps a category while rooms still name it:
	// deleting it would silently turn their badges back to the default blue.
	ErrRoomCategoryInUse = errors.New("room category is still used by rooms")
)
