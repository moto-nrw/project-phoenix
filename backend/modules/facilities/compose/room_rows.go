package compose

import (
	"context"

	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
)

// RoomRows lists retained room rows by ID.
type RoomRows interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModels.Room, error)
}

// RoomName is the ID and name of one room.
type RoomName struct {
	ID   int64
	Name string
}

// FindRoomNames names the rooms by ID; a nil row stays nil, and a nil
// listing stays nil.
func FindRoomNames(ctx context.Context, rows RoomRows, ids []int64) ([]*RoomName, error) {
	values, err := rows.FindByIDs(ctx, ids)
	if values == nil {
		return nil, err
	}
	result := make([]*RoomName, 0, len(values))
	for _, value := range values {
		if value == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &RoomName{ID: value.ID, Name: value.Name})
	}
	return result, err
}
