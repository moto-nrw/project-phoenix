package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

func validateVisit(v *studentpresence.Visit) error {
	if v.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if v.ActiveGroupID <= 0 {
		return errors.New("active group ID is required")
	}
	if v.EntryTime.IsZero() {
		return errors.New("entry time is required")
	}
	if v.ExitTime != nil && v.EntryTime.After(*v.ExitTime) {
		return errors.New("entry time must be before exit time")
	}
	return nil
}

func visitValues(rows []*studentpresence.Visit) []studentpresence.Visit {
	result := make([]studentpresence.Visit, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	return result
}

func (s *Service) FindVisit(ctx context.Context, id int64) (*studentpresence.Visit, error) {
	var result *studentpresence.Visit
	err := s.runWrite(ctx, "find_visit", func(ctx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		var stats ports.Stats
		var err error
		result, stats, err = s.store.FindVisit(ctx, id)
		return stats, err
	})
	if errors.Is(err, transactionruntime.ErrNoRows) {
		return nil, studentpresence.ErrVisitNotFound
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) ListVisits(ctx context.Context, filter studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	var rows []*studentpresence.Visit
	err := s.runQuery(ctx, filter.ForUpdate, "list_visits", func(ctx context.Context) (ports.Stats, error) {
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
		var err error
		rows, stats, err = s.store.ListVisits(ctx, filter)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return visitValues(rows), nil
}

func (s *Service) RecordVisit(ctx context.Context, value studentpresence.Visit) (studentpresence.Visit, error) {
	err := s.runWrite(ctx, "record_visit", func(txCtx context.Context) (ports.Stats, error) {
		if err := validateVisit(&value); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RecordVisit(txCtx, &value)
	})
	return value, err
}

func (s *Service) ReviseVisit(ctx context.Context, value studentpresence.Visit) (studentpresence.Visit, error) {
	err := s.runWrite(ctx, "revise_visit", func(txCtx context.Context) (ports.Stats, error) {
		if value.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		if err := validateVisit(&value); err != nil {
			return ports.Stats{}, err
		}
		return s.store.ReviseVisit(txCtx, &value)
	})
	return value, err
}

func (s *Service) DeleteVisit(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_visit", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		return s.store.DeleteVisit(txCtx, id)
	})
}

func (s *Service) CloseVisits(ctx context.Context, ids []int64, at time.Time) ([]studentpresence.Visit, error) {
	var rows []*studentpresence.Visit
	err := s.runWrite(ctx, "close_visits", func(txCtx context.Context) (ports.Stats, error) {
		if !validIDs(ids) || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid visit checkout")
		}
		var stats ports.Stats
		var err error
		rows, stats, err = s.store.CloseVisits(txCtx, ids, at)
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	return visitValues(rows), nil
}

func (s *Service) ListOpenVisitStudentIDs(ctx context.Context, hostingTenantID int64) (result []int64, err error) {
	err = s.runWrite(ctx, "list_open_visit_student_ids", func(ctx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListOpenVisitStudentIDs(ctx, hostingTenantID)
		return stats, err
	})
	return result, err
}
