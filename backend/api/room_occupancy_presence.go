package api

import (
	"context"

	facilitiesNative "github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type RoomOccupancyPresence struct {
	Source studentpresence.RoomOccupancyQuery
}

func (p RoomOccupancyPresence) ListRoomOccupancy(ctx context.Context, ids []int64) ([]facilitiesNative.RoomOccupancyFact, error) {
	rows, err := p.Source.ListRoomOccupancy(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesNative.RoomOccupancyFact, 0, len(rows))
	for _, row := range rows {
		result = append(result, facilitiesNative.RoomOccupancyFact(row))
	}
	return result, nil
}
