package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// LockOpenSupervisors holds every still-open supervisor of the group so
// completion snapshots the same rows the session end closes.
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

// LockSupervisors holds the snapshot supervisor rows during reopen so a
// concurrent staffing change cannot hide from the unchanged-row check.
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

// RestoreGroup reopens exactly the ended active group in a completion
// snapshot. A mismatch fails the surrounding recovery transaction.
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

// RestoreSupervisors reopens exactly the ended supervisors in a completion
// snapshot. A mismatch fails the surrounding recovery transaction.
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
