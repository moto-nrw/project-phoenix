package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (e engine) ListRoomOccupancy(ctx context.Context, ids []int64) ([]studentpresence.RoomOccupancy, error) {
	rows, err := e.Service.ListRoomOccupancy(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.RoomOccupancy, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.RoomOccupancy(row))
	}
	return result, nil
}
