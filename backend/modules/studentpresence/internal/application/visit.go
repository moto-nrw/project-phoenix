package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) FindVisit(ctx context.Context, id int64) (result *ports.Visit, err error) {
	err = s.run("find_visit", func() (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.FindVisit(ctx, id)
		return stats, err
	})
	return result, err
}

func (s *Service) ListVisits(ctx context.Context, filter ports.VisitFilter) (result []*ports.Visit, err error) {
	err = s.run("list_visits", func() (ports.Stats, error) {
		// Unknown IDs, including nonpositive legacy sentinels, simply do not
		// match rows. They must not suppress valid IDs in the same bulk query.
		if filter.Limit < 0 || filter.Offset < 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit filter")
		}
		if filter.ForUpdate {
			if err := s.tx.Require(ctx); err != nil {
				return ports.Stats{}, err
			}
		}
		var stats ports.Stats
		result, stats, err = s.store.ListVisits(ctx, filter)
		return stats, err
	})
	return result, err
}

func (s *Service) RecordVisit(ctx context.Context, row *ports.Visit) error {
	return s.runWrite(ctx, "record_visit", func(txCtx context.Context) (ports.Stats, error) {
		if row == nil {
			return ports.Stats{}, errors.New("student presence: visit is required")
		}
		if err := row.Validate(); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RecordVisit(txCtx, row)
	})
}

func (s *Service) ReviseVisit(ctx context.Context, row *ports.Visit) error {
	return s.runWrite(ctx, "revise_visit", func(txCtx context.Context) (ports.Stats, error) {
		if row == nil || row.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		if err := row.Validate(); err != nil {
			return ports.Stats{}, err
		}
		return s.store.ReviseVisit(txCtx, row)
	})
}

func (s *Service) DeleteVisit(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_visit", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		return s.store.DeleteVisit(txCtx, id)
	})
}

func (s *Service) CloseVisits(ctx context.Context, ids []int64, at time.Time) (result []*ports.Visit, err error) {
	err = s.runWrite(ctx, "close_visits", func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(ids) || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid visit checkout")
		}
		var stats ports.Stats
		result, stats, err = s.store.CloseVisits(txCtx, ids, at)
		return stats, err
	})
	return result, err
}
