package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListRoomSessionHistory(ctx context.Context, roomID int64, start, end time.Time, staffID *int64) (result []studentpresence.RoomSessionHistory, err error) {
	err = s.runWrite(ctx, "list_room_session_history", func(ctx context.Context) (ports.Stats, error) {
		if roomID <= 0 || start.After(end) || staffID != nil && *staffID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid room session history query")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListRoomSessionHistory(ctx, roomID, start, end, staffID)
		return stats, err
	})
	return listResult(result, err)
}
