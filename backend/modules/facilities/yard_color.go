package facilities

import (
	"context"
	"fmt"
)

// SchulhofRoomColor returns the canonical system yard's configured color.
// Legacy case-insensitive name collisions must not select an ordinary room.
func SchulhofRoomColor(ctx context.Context, rooms Query) (*string, error) {
	if rooms == nil {
		return nil, nil
	}
	name := SchulhofRoomName
	rows, err := rooms.ListRooms(ctx, RoomFilter{Name: &name})
	if err != nil {
		return nil, fmt.Errorf("list Schulhof rooms: %w", err)
	}
	for _, room := range rows {
		if room.Name != SchulhofRoomName || !room.IsSystem {
			continue
		}
		if room.Color == nil || *room.Color == "" {
			return nil, nil
		}
		return room.Color, nil
	}
	return nil, nil
}
