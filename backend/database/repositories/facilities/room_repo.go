package facilities

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// RoomRepository implements facilities.RoomRepository
type RoomRepository struct {
	db *bun.DB
}

// NewRoomRepository creates a new room repository
func NewRoomRepository(db *bun.DB) facilities.RoomRepository {
	return &RoomRepository{db: db}
}

// Create inserts a new room into the database
func (r *RoomRepository) Create(ctx context.Context, room *facilities.Room) error {
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
func (r *RoomRepository) FindByID(ctx context.Context, id interface{}) (*facilities.Room, error) {
	room := new(facilities.Room)
	err := r.db.NewSelect().Model(room).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return room, nil
}

// FindByName retrieves a room by its name
func (r *RoomRepository) FindByName(ctx context.Context, name string) (*facilities.Room, error) {
	room := new(facilities.Room)
	err := r.db.NewSelect().Model(room).Where("name = ?", name).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return room, nil
}

// FindByBuilding retrieves rooms by building
func (r *RoomRepository) FindByBuilding(ctx context.Context, building string) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
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
func (r *RoomRepository) FindByCategory(ctx context.Context, category string) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
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
func (r *RoomRepository) FindByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
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
func (r *RoomRepository) Update(ctx context.Context, room *facilities.Room) error {
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
func (r *RoomRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*facilities.Room)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves rooms matching the filters
func (r *RoomRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
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