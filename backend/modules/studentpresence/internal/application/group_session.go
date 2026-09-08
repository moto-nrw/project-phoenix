package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) LockGroup(ctx context.Context, groupID int64) (result ports.LiveGroup, err error) {
	err = s.run("lock_group", func() (ports.Stats, error) {
		if groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid active group ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.LockGroup(ctx, groupID)
		return stats, err
	})
	return result, err
}

func (s *Service) EndGroupSessions(ctx context.Context, groupIDs []int64, at time.Time, endDate ports.Date) (result ports.EndedGroupSessions, err error) {
	err = s.run("end_group_sessions", func() (ports.Stats, error) {
		if !validIDs(groupIDs) || at.IsZero() || endDate.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid bulk group session end")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.EndGroupSessions(ctx, groupIDs, at, endDate)
		return stats, err
	})
	return result, err
}

func (s *Service) EndGroupSession(ctx context.Context, groupID int64, at time.Time, endDate ports.Date) (result ports.EndedGroupSession, err error) {
	err = s.run("end_group_session", func() (ports.Stats, error) {
		if groupID <= 0 || at.IsZero() || endDate.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid group session end")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.EndGroupSession(ctx, groupID, at, endDate)
		return stats, err
	})
	return result, err
}
