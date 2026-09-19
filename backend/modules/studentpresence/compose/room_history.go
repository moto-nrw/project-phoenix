package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (e engine) ListRoomSessionHistory(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]studentpresence.RoomSessionHistory, error) {
	rows, err := e.Service.ListRoomSessionHistory(ctx, roomID, start, end, staffID)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.RoomSessionHistory, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.RoomSessionHistory(row))
	}
	return result, nil
}
