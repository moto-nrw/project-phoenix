package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/services/active"
)

func historyRoomNames(rooms facilities.Query) active.HistoryRoomReader {
	return func(ctx context.Context, ids []int64) (map[int64]string, error) {
		rows, err := rooms.ListRoomsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make(map[int64]string, len(rows))
		for _, row := range rows {
			result[row.ID] = row.Name
		}
		return result, nil
	}
}
