package services

import (
	"context"

	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type RoomOccupancyPresence struct {
	Source studentpresence.RoomOccupancyQuery
}

func (p RoomOccupancyPresence) ListRoomOccupancy(ctx context.Context, ids []int64) ([]facilitiesLegacy.RoomOccupancyFact, error) {
	rows, err := p.Source.ListRoomOccupancy(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesLegacy.RoomOccupancyFact, 0, len(rows))
	for _, row := range rows {
		result = append(result, facilitiesLegacy.RoomOccupancyFact(row))
	}
	return result, nil
}
