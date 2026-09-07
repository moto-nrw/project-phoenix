package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type Service struct {
	store   ports.Store
	tx      ports.Transaction
	observe func(ports.Observation)
}

func New(store ports.Store, tx ports.Transaction, observe func(ports.Observation)) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("student presence: all application dependencies are required")
	}
	return &Service{store: store, tx: tx, observe: observe}
}

func (s *Service) run(operation string, fn func() (ports.Stats, error)) error {
	started := time.Now()
	stats, err := fn()
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func validIDs(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return false
		}
	}
	return true
}

func (s *Service) ListOpenPresence(ctx context.Context, ids []int64) (result []int64, err error) {
	err = s.run("list_open_presence", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListOpenPresence(ctx, ids)
		return stats, err
	})
	return result, err
}

func (s *Service) LockOpenVisits(ctx context.Context, groupID int64) error {
	return s.run("lock_open_visits", func() (ports.Stats, error) {
		if groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid active group ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockOpenVisits(ctx, groupID)
	})
}

func (s *Service) RestoreVisits(ctx context.Context, ids []int64) error {
	return s.run("restore_visits", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RestoreVisits(ctx, ids)
	})
}

func (s *Service) CloseOpenPresence(ctx context.Context, ids []int64, at time.Time) (int64, error) {
	var result int64
	err := s.run("close_open_presence", func() (ports.Stats, error) {
		if !validIDs(ids) || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid presence close")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		stats, err := s.store.CloseOpenPresence(ctx, ids, at)
		if err == nil {
			result = stats.Rows
		}
		return stats, err
	})
	return result, err
}

func (s *Service) LatestPresenceDate(ctx context.Context, studentID int64) (result *string, err error) {
	err = s.run("latest_presence_date", func() (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.LatestPresenceDate(ctx, studentID)
		return stats, err
	})
	return result, err
}

func (s *Service) CountAttendanceRecords(ctx context.Context, studentID int64) (result int, err error) {
	err = s.run("count_attendance_records", func() (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.CountAttendanceRecords(ctx, studentID)
		return stats, err
	})
	return result, err
}

func (s *Service) LockOpenPresence(ctx context.Context, ids []int64) error {
	return s.run("lock_open_presence", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockOpenPresence(ctx, ids)
	})
}
