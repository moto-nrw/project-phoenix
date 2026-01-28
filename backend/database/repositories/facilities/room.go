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

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableFacilitiesRooms           = "facilities.rooms"
	tableExprFacilitiesRoomsAsRoom = "facilities.rooms AS room"
)

// RoomRepository implements facilities.RoomRepository interface
type RoomRepository struct {
	*base.Repository[*facilities.Room]
	db *bun.DB
}

// NewRoomRepository creates a new RoomRepository
func NewRoomRepository(db *bun.DB) facilities.RoomRepository {
	return &RoomRepository{
		Repository: base.NewRepository[*facilities.Room](db, tableFacilitiesRooms, "Room"),
		db:         db,
	}
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

	// Use the base Create method which now uses ModelTableExpr
	return r.Repository.Create(ctx, room)
}

// Update overrides the base Update method for schema consistency
func (r *RoomRepository) Update(ctx context.Context, room *facilities.Room) error {
	if room == nil {
		return fmt.Errorf("room cannot be nil")
	}

	// Validate room
	if err := room.Validate(); err != nil {
		return err
	}

	// Get the query builder - detect if we're in a transaction
	query := r.db.NewUpdate().
		Model(room).
		Where("id = ?", room.ID).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom)

	// Extract transaction from context if it exists
	if tx, ok := ctx.Value("tx").(*bun.Tx); ok && tx != nil {
		// Use the transaction if available
		query = tx.NewUpdate().
			Model(room).
			Where("id = ?", room.ID).
			ModelTableExpr(tableExprFacilitiesRoomsAsRoom)
	}

	// Execute the query
	_, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// FindByName retrieves a room by its name
func (r *RoomRepository) FindByName(ctx context.Context, name string) (*facilities.Room, error) {
	room := new(facilities.Room)
	err := r.db.NewSelect().
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
		Where("LOWER(name) = LOWER(?)", name).
		Scan(ctx, room)

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
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
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
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
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
	query := r.db.NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom)

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

// List retrieves rooms matching the provided filters
// Note: This implementation still uses the old map[string]interface{} filter system
// but should be migrated to QueryOptions in the future
func (r *RoomRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := r.db.NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = applyRoomFilter(query, field, value)
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

// applyRoomFilter applies a single filter to the query based on field name
func applyRoomFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "name":
		return applyCaseInsensitiveExactMatch(query, "name", value)
	case "name_like":
		return applyCaseInsensitiveLikeMatch(query, "name", value)
	case "building":
		return applyCaseInsensitiveExactMatch(query, "building", value)
	case "building_like":
		return applyCaseInsensitiveLikeMatch(query, "building", value)
	case "category":
		return applyCaseInsensitiveExactMatch(query, "category", value)
	case "min_capacity":
		return query.Where("capacity >= ?", value)
	case "max_capacity":
		return query.Where("capacity <= ?", value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyCaseInsensitiveExactMatch applies case-insensitive exact match filter
func applyCaseInsensitiveExactMatch(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+column+") = LOWER(?)", strValue)
	}
	return query.Where(column+" = ?", value)
}

// applyCaseInsensitiveLikeMatch applies case-insensitive LIKE filter
func applyCaseInsensitiveLikeMatch(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where("LOWER("+column+") LIKE LOWER(?)", "%"+strValue+"%")
	}
	return query
}

// ListWithOptions retrieves rooms with the new type-safe query options system
func (r *RoomRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*facilities.Room, error) {
	var rooms []*facilities.Room
	query := r.db.NewSelect().
		Model(&rooms).
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom) // Use proper table alias

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
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
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
		ModelTableExpr(tableExprFacilitiesRoomsAsRoom).
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
