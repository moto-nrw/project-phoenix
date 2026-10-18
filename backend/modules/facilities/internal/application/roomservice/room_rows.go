package facilities

import (
	"context"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
)

// FindRoomNames names the rooms by ID; a nil row stays nil, and a nil
// listing stays nil.
func FindRoomNames(ctx context.Context, rows facilitiesModule.RoomRows, ids []int64) ([]*facilitiesModule.RoomName, error) {
	values, err := rows.FindByIDs(ctx, ids)
	if values == nil {
		return nil, err
	}
	result := make([]*facilitiesModule.RoomName, 0, len(values))
	for _, value := range values {
		if value == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &facilitiesModule.RoomName{ID: value.ID, Name: value.Name})
	}
	return result, err
}
