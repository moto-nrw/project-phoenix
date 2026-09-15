package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListOpenRoomSessions(ctx context.Context, ids []int64) (result []ports.OpenRoomSession, err error) {
	err = s.run("list_open_room_sessions", func() (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListOpenRoomSessions(ctx, ids)
		return stats, err
	})
	return result, err
}
