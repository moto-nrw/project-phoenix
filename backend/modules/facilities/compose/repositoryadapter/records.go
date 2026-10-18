package repositoryadapter

import (
	"context"
	"errors"

	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Records serves the retained room rows the legacy readers ask for over the
// Facilities queries alone, so a root can hand them out before the room
// deletion exists (#4080). It writes no room; the Repository reads through
// it.
type Records struct{ rooms facilities.Query }

func NewRecords(rooms facilities.Query) *Records {
	if rooms == nil {
		panic("room records adapter: Facilities query is required")
	}
	return &Records{rooms: rooms}
}

// query is unbound only for a Repository before Bind.
func (r *Records) query() (facilities.Query, error) {
	if r.rooms == nil {
		return nil, errors.New("room repository adapter: Facilities capability is not bound")
	}
	return r.rooms, nil
}

func (r *Records) FindByID(ctx context.Context, rawID any) (*facilitiesModels.Room, error) {
	id, err := roomID(rawID)
	if err != nil {
		return nil, err
	}
	rooms, err := r.query()
	if err != nil {
		return nil, err
	}
	room, err := rooms.FindRoom(ctx, id)
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
	rooms, err := r.query()
	if err != nil {
		return nil, err
	}
	values, err := rooms.ListRooms(ctx, filter)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModels.Room, error) {
	rooms, err := r.query()
	if err != nil {
		return nil, err
	}
	values, err := rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDForUpdate(ctx context.Context, id int64) (*facilitiesModels.Room, error) {
	rooms, err := r.query()
	if err != nil {
		return nil, err
	}
	room, err := rooms.FindRoomForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}

func (r *Records) FindByName(ctx context.Context, name string) (*facilitiesModels.Room, error) {
	rooms, err := r.query()
	if err != nil {
		return nil, err
	}
	room, err := rooms.FindRoomByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}
