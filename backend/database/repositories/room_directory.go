package repositories

import (
	"context"
	"fmt"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/uptrace/bun"
)

// NewFacilities composes the room owner behind the legacy composition seam
// for graphs that do not record observations. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewFacilities(db *bun.DB) (facilitiesModule.Capability, error) {
	return facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:           db,
		DeletionLock: func(context.Context) error { return nil },
		// The legacy repository accepted deletes without service-level guards.
		// Keep that compatibility path usable; production binds its fully
		// validated Facilities capability from the API composition root.
		DeletionGuard: func(context.Context, int64) error { return nil },
		Observe:       func(facilitiesCompose.Observation) {},
	})
}

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
			Category: room.Category, Color: room.Color, IsSystem: room.IsSystem,
			IsOpenRoom: room.IsOpenRoom,
		})
	}
	return result, nil
}
