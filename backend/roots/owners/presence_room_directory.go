package owners

import (
	"context"
	"fmt"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

// activeRoomDirectory adapts the room owner for the retained session
// repository (#2665, #3214).
type activeRoomDirectory struct{ rooms facilitiesModule.Query }

func (d activeRoomDirectory) ListRoomsByID(ctx context.Context, ids []int64) ([]presenceCompose.DirectoryRoom, error) {
	if d.rooms == nil {
		return nil, fmt.Errorf("active room directory: no room owner bound")
	}
	rooms, err := d.rooms.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]presenceCompose.DirectoryRoom, 0, len(rooms))
	for _, room := range rooms {
		result = append(result, presenceCompose.DirectoryRoom{
			ID: room.ID, TenantID: room.TenantID, CreatedAt: room.CreatedAt, UpdatedAt: room.UpdatedAt,
			Name: room.Name, Building: room.Building, Floor: room.Floor, Capacity: room.Capacity,
			// Presence shows the room's badge color (#4352).
			Category: room.Category, Color: room.BadgeColor(), IsSystem: room.IsSystem,
			IsOpenRoom: room.IsOpenRoom,
		})
	}
	return result, nil
}
