package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListRoomOccupancy(ctx context.Context, ids []int64) (result []ports.RoomOccupancy, err error) {
	err = s.run("list_room_occupancy", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListRoomOccupancy(ctx, ids)
		return stats, err
	})
	return result, err
}
