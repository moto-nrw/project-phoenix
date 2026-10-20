package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// LockStaffSupervision returns the supervision IDs active on the given school
// day and holds their row locks until the caller's tenant transaction ends.
// The caller must also serialize staff lifecycle changes with assignment
// writers: these row locks alone do not prevent new supervision rows.
func (s *Service) LockStaffSupervision(ctx context.Context, staffID int64, value string) (result []int64, err error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return nil, err
	}
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
