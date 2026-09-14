package services

import (
	"context"
	"time"

	facilitiesLegacy "github.com/moto-nrw/project-phoenix/modules/facilities/compose/legacy"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type roomHistoryPresence struct {
	source studentpresence.RoomHistoryQuery
}

func (p roomHistoryPresence) ListRoomSessionHistory(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) ([]facilitiesLegacy.RoomSessionFact, error) {
	rows, err := p.source.ListRoomSessionHistory(ctx, roomID, start, end, staffID)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesLegacy.RoomSessionFact, 0, len(rows))
	for _, row := range rows {
		result = append(result, facilitiesLegacy.RoomSessionFact(row))
	}
	return result, nil
}
