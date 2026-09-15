package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (e engine) ListOpenRoomSessions(ctx context.Context, ids []int64) ([]studentpresence.OpenRoomSession, error) {
	rows, err := e.Service.ListOpenRoomSessions(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.OpenRoomSession, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.OpenRoomSession(row))
	}
	return result, nil
}
