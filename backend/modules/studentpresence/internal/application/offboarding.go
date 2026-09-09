package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) LockStaffSupervision(ctx context.Context, staffID int64, date ports.Date) (result []int64, err error) {
	err = s.run("lock_staff_supervision", func() (ports.Stats, error) {
		if staffID <= 0 || date.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid staff supervision query")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.LockStaffSupervision(ctx, staffID, date)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
