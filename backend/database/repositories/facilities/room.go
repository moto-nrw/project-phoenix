// backend/database/repositories/facilities/room.go
package facilities

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/uptrace/bun"
)

// RoomRepository implements facilities.RoomRepository interface
type RoomRepository struct {
	*base.Repository[*facilities.Room]
	db *bun.DB
}

// NewRoomRepository creates a new RoomRepository
func NewRoomRepository(db *bun.DB) facilities.RoomRepository {
	return &RoomRepository{
		Repository: base.NewRepository[*facilities.Room](db, "facilities.rooms", "Room"),
		db:         db,
	}
}

// FindByName retrieves a room by its name
func (r *RoomRepository) FindByName(ctx context.Context, name string) (*facilities.Room, error) {
	room := new(facilities.Room)
	err := r.db.NewSelect().
		Model(room).
		Where("LOWER(name) = LOWER(?)", name).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by name",
			Err: err,
		}
	}

	return room, nil
}

// FindByBuilding retrieves rooms by building
func (r *RoomRepository) FindByBuilding(ctx context.Context, building string) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	err := r.db.NewSelect().
		Model(&rooms).
		Where("LOWER(building) = LOWER(?)", building).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by building",
			Err: err,
		}
	}

	return rooms, nil
}

// FindByCategory retrieves rooms by category
func (r *RoomRepository) FindByCategory(ctx context.Context, category string) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	err := r.db.NewSelect().
		Model(&rooms).
		Where("LOWER(category) = LOWER(?)", category).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by category",
			Err: err,
		}
	}

	return rooms, nil
}

// FindByFloor retrieves rooms by building and floor
func (r *RoomRepository) FindByFloor(ctx context.Context, building string, floor int) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := r.db.NewSelect().Model(&rooms)

	if building != "" {
		query = query.Where("LOWER(building) = LOWER(?)", building)
	}

	query = query.Where("floor = ?", floor)

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by floor",
			Err: err,
		}
	}

	return rooms, nil
}

// Create overrides the base Create method to handle validation
func (r *RoomRepository) Create(ctx context.Context, room *facilities.Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	// Validate room
	if err := room.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, room)
}

// Update overrides the base Update method to handle validation
func (r *RoomRepository) Update(ctx context.Context, room *facilities.Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	// Validate room
	if err := room.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, room)
}

// List retrieves rooms matching the provided filters
// Note: This implementation still uses the old map[string]interface{} filter system
// but should be migrated to QueryOptions in the future
func (r *RoomRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := r.db.NewSelect().Model(&rooms)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "name":
				// Case-insensitive name search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(name) = LOWER(?)", strValue)
				} else {
					query = query.Where("name = ?", value)
				}
			case "name_like":
				// Case-insensitive name pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "building":
				// Case-insensitive building search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(building) = LOWER(?)", strValue)
				} else {
					query = query.Where("building = ?", value)
				}
			case "building_like":
				// Case-insensitive building pattern search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(building) LIKE LOWER(?)", "%"+strValue+"%")
				}
			case "category":
				// Case-insensitive category search
				if strValue, ok := value.(string); ok {
					query = query.Where("LOWER(category) = LOWER(?)", strValue)
				} else {
					query = query.Where("category = ?", value)
				}
			case "min_capacity":
				query = query.Where("capacity >= ?", value)
			case "max_capacity":
				query = query.Where("capacity <= ?", value)
			default:
				// Default to exact match for other fields
				query = query.Where("? = ?", bun.Ident(field), value)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return rooms, nil
}

// ListWithOptions retrieves rooms with the new type-safe query options system
func (r *RoomRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := r.db.NewSelect().Model(&rooms)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return rooms, nil
}

// FindWithCapacity retrieves rooms with at least the specified capacity
func (r *RoomRepository) FindWithCapacity(ctx context.Context, minCapacity int) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	err := r.db.NewSelect().
		Model(&rooms).
		Where("capacity >= ?", minCapacity).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with capacity",
			Err: err,
		}
	}

	return rooms, nil
}

// SearchByText performs a text search across multiple room fields
func (r *RoomRepository) SearchByText(ctx context.Context, searchText string) ([]*facilities.Room, error) {
	if searchText == "" {
		return []*facilities.Room{}, nil
	}

	var rooms []*facilities.Room
	searchPattern := "%" + strings.ToLower(searchText) + "%"

	err := r.db.NewSelect().
		Model(&rooms).
		Where("LOWER(name) LIKE ? OR LOWER(building) LIKE ? OR LOWER(category) LIKE ?",
			searchPattern, searchPattern, searchPattern).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "search by text",
			Err: err,
		}
	}

	return rooms, nil
}
