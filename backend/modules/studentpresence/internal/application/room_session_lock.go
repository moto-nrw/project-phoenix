package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) LockRoomSessionWrites(ctx context.Context, roomID int64) error {
	return s.run("lock_room_session_writes", func() (ports.Stats, error) {
		if roomID <= 0 {
			return ports.Stats{}, errors.New("lock room session writes: room ID must be positive")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockRoomSessionWrites(ctx, roomID)
	})
}
