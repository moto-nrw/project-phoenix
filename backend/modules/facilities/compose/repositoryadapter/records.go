package repositoryadapter

import (
	"context"

	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Records serves the retained room rows the legacy readers ask for over the
// Facilities queries alone, so a root can hand them out before the room
// deletion exists (#4080). It writes no room.
type Records struct{ rooms facilities.Query }

func NewRecords(rooms facilities.Query) *Records {
	if rooms == nil {
		panic("room records adapter: Facilities query is required")
	}
	return &Records{rooms: rooms}
}

func (r *Records) FindByID(ctx context.Context, rawID any) (*facilitiesModels.Room, error) {
	id, err := roomID(rawID)
	if err != nil {
		return nil, err
	}
	room, err := r.rooms.FindRoom(ctx, id)
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
	values, err := r.rooms.ListRooms(ctx, filter)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModels.Room, error) {
	values, err := r.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return toLegacyPointers(values), nil
}

func (r *Records) FindByIDForUpdate(ctx context.Context, id int64) (*facilitiesModels.Room, error) {
	room, err := r.rooms.FindRoomForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}

func (r *Records) FindByName(ctx context.Context, name string) (*facilitiesModels.Room, error) {
	room, err := r.rooms.FindRoomByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return toLegacyPointer(room), nil
}
