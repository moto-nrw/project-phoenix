package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) LockOpenSupervisors(ctx context.Context, groupID int64) error {
	return s.run("lock_open_supervisors", func() (ports.Stats, error) {
		if groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid active group ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockOpenSupervisors(ctx, groupID)
	})
}

func (s *Service) LockSupervisors(ctx context.Context, ids []int64) error {
	return s.run("lock_supervisors", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid supervisor ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockSupervisors(ctx, ids)
	})
}

func (s *Service) RestoreGroup(ctx context.Context, groupID int64, now time.Time) error {
	return s.run("restore_group", func() (ports.Stats, error) {
		if groupID <= 0 || now.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid active group restore")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RestoreGroup(ctx, groupID, now)
	})
}

func (s *Service) RestoreSupervisors(ctx context.Context, ids []int64) error {
	return s.run("restore_supervisors", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid supervisor ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RestoreSupervisors(ctx, ids)
	})
}
