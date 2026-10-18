// Package repositoryadapter keeps legacy repository consumers on the
// Facilities capability while they migrate to its Query and Command façades.
package repositoryadapter

import (
	"context"
	"errors"
	"fmt"

	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// roomQuery and roomCommand name the Facilities halves the adapters embed;
// the fields stay unexported, so only the constructors bind them.
type (
	roomQuery   = facilities.Query
	roomCommand = facilities.Command
)

// Repository serves the retained room repository contract: the reads of its
// Records and the writes over the Facilities capability. Only New builds it.
type Repository struct {
	*Records
	roomCommand
}

// New binds the repository to rooms; a missing capability panics.
func New(rooms facilities.Capability) *Repository {
	if rooms == nil {
		panic("room repository adapter: Facilities capability is required")
	}
	return &Repository{Records: NewRecords(rooms), roomCommand: rooms}
}

// Records serves the retained room rows the legacy readers ask for over the
// Facilities queries alone, so a root can hand them out before the room
// deletion exists (#4080). It writes no room; the Repository reads through
// it. Only NewRecords builds it.
type Records struct{ roomQuery }

func NewRecords(rooms facilities.Query) *Records {
	if rooms == nil {
		panic("room records adapter: Facilities query is required")
	}
	return &Records{roomQuery: rooms}
}

func (r *Records) FindByID(ctx context.Context, rawID any) (*facilitiesModels.Room, error) {
	id, err := roomID(rawID)
	if err != nil {
		return nil, err
	}
	room, err := r.FindRoom(ctx, id)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}

func (r *Records) List(ctx context.Context, filters map[string]any) ([]*facilitiesModels.Room, error) {
	filter, err := roomFilter(filters)
	if err != nil {
		return nil, err
	}
	values, err := r.ListRooms(ctx, filter)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModels.Room, error) {
	values, err := r.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDForUpdate(ctx context.Context, id int64) (*facilitiesModels.Room, error) {
	room, err := r.FindRoomForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}

func (r *Records) FindByName(ctx context.Context, name string) (*facilitiesModels.Room, error) {
	room, err := r.FindRoomByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}

func (r *Repository) Create(ctx context.Context, room *facilitiesModels.Room) error {
	if room == nil {
		return errors.New("room cannot be nil")
	}
	created, err := r.CreateRoom(ctx, facilities.CreateRoom{
		Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
		Category: room.Category, Color: room.Color, IsSystem: room.IsSystem,
		IsOpenRoom: room.IsOpenRoom,
	})
	if err == nil {
		applyPublic(room, created)
	}
	return err
}

func (r *Repository) Update(ctx context.Context, room *facilitiesModels.Room) error {
	if room == nil {
		return errors.New("room cannot be nil")
	}
	updated, err := r.UpdateRoom(ctx, facilities.UpdateRoom{
		ID: room.ID, Name: room.Name, Building: room.Building, Floor: room.Floor,
		Capacity: room.Capacity, Category: room.Category, Color: room.Color,
		// Legacy callers read-modify-write a whole room, so the loaded
		// release value is passed back explicitly. Sending nil here would
		// make every legacy save silently skip the column instead of
		// preserving what the caller actually holds.
		IsOpenRoom: &room.IsOpenRoom,
	})
	if err == nil {
		applyPublic(room, updated)
	}
	return err
}

func (r *Repository) Delete(ctx context.Context, rawID any) error {
	id, err := roomID(rawID)
	if err != nil {
		return err
	}
	return r.DeleteRoom(ctx, id)
}

func roomID(value any) (int64, error) {
	switch id := value.(type) {
	case int64:
		return id, nil
	case int:
		return int64(id), nil
	default:
		return 0, fmt.Errorf("room ID must be an integer, got %T", value)
	}
}

func roomFilter(filters map[string]any) (facilities.RoomFilter, error) {
	filter := facilities.RoomFilter{}
	for field, raw := range filters {
		if err := applyRoomFilter(&filter, field, raw); err != nil {
			return facilities.RoomFilter{}, err
		}
	}
	return filter, nil
}

func applyRoomFilter(filter *facilities.RoomFilter, field string, raw any) error {
	switch field {
	case "name", "name_like", "building", "building_like", "category":
		value, ok := raw.(string)
		if !ok {
			return fmt.Errorf("room filter %s must be a string", field)
		}
		setStringFilter(filter, field, &value)
	case "floor", "min_capacity", "max_capacity":
		value, ok := raw.(int)
		if !ok {
			return fmt.Errorf("room filter %s must be an integer", field)
		}
		setIntegerFilter(filter, field, &value)
	default:
		return fmt.Errorf("unsupported room filter %q", field)
	}
	return nil
}

func setStringFilter(filter *facilities.RoomFilter, field string, value *string) {
	switch field {
	case "name":
		filter.Name = value
	case "name_like":
		filter.NameContains = value
	case "building":
		filter.Building = value
	case "building_like":
		filter.BuildingContains = value
	case "category":
		filter.Category = value
	}
}

func setIntegerFilter(filter *facilities.RoomFilter, field string, value *int) {
	switch field {
	case "floor":
		filter.Floor = value
	case "min_capacity":
		filter.MinimumCapacity = value
	case "max_capacity":
		filter.MaximumCapacity = value
	}
}

func toLegacyPointers(values []facilities.Room) []*facilitiesModels.Room {
	result := make([]*facilitiesModels.Room, len(values))
	for index := range values {
		result[index] = toLegacyPointer(values[index])
	}
	return result
}

func toLegacyPointer(value facilities.Room) *facilitiesModels.Room {
	room := &facilitiesModels.Room{}
	applyPublic(room, value)
	return room
}

func applyPublic(target *facilitiesModels.Room, source facilities.Room) {
	target.ID, target.TenantID = source.ID, source.TenantID
	target.CreatedAt, target.UpdatedAt = source.CreatedAt, source.UpdatedAt
	target.Name, target.Building = source.Name, source.Building
	target.Floor, target.Capacity = source.Floor, source.Capacity
	target.Category, target.Color, target.IsSystem = source.Category, source.Color, source.IsSystem
	target.IsOpenRoom = source.IsOpenRoom
}
