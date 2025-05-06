package facilities

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Room represents a room in the facilities
type Room struct {
	base.Model
	Name     string `bun:"name,notnull,unique" json:"name"`
	Building string `bun:"building" json:"building,omitempty"`
	Floor    int    `bun:"floor,notnull,default:0" json:"floor"`
	Capacity int    `bun:"capacity,notnull,default:0" json:"capacity"`
	Category string `bun:"category,notnull,default:'Other'" json:"category"`
	Color    string `bun:"color,notnull,default:'#FFFFFF'" json:"color"`

	// Relations
	Occupancies []*RoomOccupancy `bun:"rel:has-many,join:id=room_id" json:"occupancies,omitempty"`
	History     []*RoomHistory   `bun:"rel:has-many,join:id=room_id" json:"history,omitempty"`
}

// TableName returns the table name for the Room model
func (r *Room) TableName() string {
	return "facilities.rooms"
}

// GetID returns the room ID
func (r *Room) GetID() interface{} {
	return r.ID
}

// GetCreatedAt returns the creation timestamp
func (r *Room) GetCreatedAt() time.Time {
	return r.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (r *Room) GetUpdatedAt() time.Time {
	return r.UpdatedAt
}

// Validate validates the room fields
func (r *Room) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("room name is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (r *Room) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := r.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	r.Name = strings.TrimSpace(r.Name)
	r.Building = strings.TrimSpace(r.Building)
	r.Category = strings.TrimSpace(r.Category)
	r.Color = strings.TrimSpace(r.Color)

	// Set default category if empty
	if r.Category == "" {
		r.Category = "Other"
	}

	// Set default color if empty
	if r.Color == "" {
		r.Color = "#FFFFFF"
	}

	return nil
}

// RoomRepository defines operations for working with rooms
type RoomRepository interface {
	base.Repository[*Room]
	FindByName(ctx context.Context, name string) (*Room, error)
	FindByBuilding(ctx context.Context, building string) ([]*Room, error)
	FindByCategory(ctx context.Context, category string) ([]*Room, error)
	FindByFloor(ctx context.Context, building string, floor int) ([]*Room, error)
}

// DefaultRoomRepository is the default implementation of RoomRepository
type DefaultRoomRepository struct {
	db *bun.DB
}

// NewRoomRepository creates a new room repository
func NewRoomRepository(db *bun.DB) RoomRepository {
	return &DefaultRoomRepository{db: db}
}

// Create inserts a new room into the database
func (r *DefaultRoomRepository) Create(ctx context.Context, room *Room) error {
	if err := room.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(room).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a room by its ID
func (r *DefaultRoomRepository) FindByID(ctx context.Context, id interface{}) (*Room, error) {
	room := new(Room)
	err := r.db.NewSelect().Model(room).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return room, nil
}

// FindByName retrieves a room by its name
func (r *DefaultRoomRepository) FindByName(ctx context.Context, name string) (*Room, error) {
	room := new(Room)
	err := r.db.NewSelect().Model(room).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return room, nil
}

// FindByBuilding retrieves rooms by building
func (r *DefaultRoomRepository) FindByBuilding(ctx context.Context, building string) ([]*Room, error) {
	var rooms []*Room
	err := r.db.NewSelect().
		Model(&rooms).
		Where("building = ?", building).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_building", Err: err}
	}
	return rooms, nil
}

// FindByCategory retrieves rooms by category
func (r *DefaultRoomRepository) FindByCategory(ctx context.Context, category string) ([]*Room, error) {
	var rooms []*Room
	err := r.db.NewSelect().
		Model(&rooms).
		Where("category = ?", category).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_category", Err: err}
	}
	return rooms, nil
}

// FindByFloor retrieves rooms by building and floor
func (r *DefaultRoomRepository) FindByFloor(ctx context.Context, building string, floor int) ([]*Room, error) {
	var rooms []*Room
	query := r.db.NewSelect().Model(&rooms).Where("floor = ?", floor)

	if building != "" {
		query = query.Where("building = ?", building)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_floor", Err: err}
	}
	return rooms, nil
}

// Update updates an existing room
func (r *DefaultRoomRepository) Update(ctx context.Context, room *Room) error {
	if err := room.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(room).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a room
func (r *DefaultRoomRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Room)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves rooms matching the filters
func (r *DefaultRoomRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Room, error) {
	var rooms []*Room
	query := r.db.NewSelect().Model(&rooms)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return rooms, nil
}
