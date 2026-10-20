package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

type roomDirectory struct{ query timetable.RoomDirectory }

func (d roomDirectory) LockRoomsByID(ctx context.Context, ids []int64) ([]timetable.RoomRef, domain.OperationStats, error) {
	values, err := d.query.LockRoomsByID(ctx, ids)
	result := make([]timetable.RoomRef, 0, len(values))
	for _, value := range values {
		result = append(result, timetable.RoomRef(value))
	}
	return result, domain.OperationStats{}, err
}

func NewRoomDirectory(query timetable.RoomDirectory) roomDirectory {
	return roomDirectory{query: query}
}
