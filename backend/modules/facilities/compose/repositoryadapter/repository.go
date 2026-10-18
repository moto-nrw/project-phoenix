// Package repositoryadapter keeps legacy repository consumers on the
// Facilities capability while they migrate to its Query and Command façades.
package repositoryadapter

import (
	"context"
	"fmt"

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

func (r *Records) FindByID(ctx context.Context, rawID any) (*facilities.Room, error) {
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

func (r *Records) List(ctx context.Context, filters map[string]any) ([]*facilities.Room, error) {
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

func (r *Records) FindByIDs(ctx context.Context, ids []int64) ([]*facilities.Room, error) {
	values, err := r.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDForUpdate(ctx context.Context, id int64) (*facilities.Room, error) {
	room, err := r.FindRoomForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}

func (r *Records) FindByName(ctx context.Context, name string) (*facilities.Room, error) {
	room, err := r.FindRoomByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
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

func toLegacyPointers(values []facilities.Room) []*facilities.Room {
	result := make([]*facilities.Room, len(values))
	for index := range values {
		result[index] = toLegacyPointer(values[index])
	}
	return result
}

func toLegacyPointer(value facilities.Room) *facilities.Room { return &value }
