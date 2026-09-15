package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func (s *Service) ListCombinedGroups(ctx context.Context, filter ports.CombinedGroupFilter) (result []ports.CombinedGroup, err error) {
	err = s.run("list_combined_groups", func() (ports.Stats, error) {
		if filter.Limit < 0 || filter.Offset < 0 || filter.ID != nil && *filter.ID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid combination list options")
		}
		if filter.From != nil && filter.Until != nil && filter.From.After(*filter.Until) {
			return ports.Stats{}, errors.New("student presence: invalid combination time range")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListCombinedGroups(ctx, filter)
		return stats, err
	})
	return result, err
}
func (s *Service) EndCombination(ctx context.Context, id int64, at time.Time) error {
	return s.runWrite(ctx, "end_combination", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid combination end")
		}
		return s.store.EndCombination(txCtx, id, at)
	})
}

func (s *Service) GetCombinedGroup(ctx context.Context, id int64) (result ports.CombinedGroup, err error) {
	err = s.run("get_combined_group", func() (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid combination ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.GetCombinedGroup(ctx, id)
		return stats, err
	})
	return result, err
}
func (s *Service) RecordCombination(ctx context.Context, start time.Time, end *time.Time) (result ports.CombinedGroup, err error) {
	err = s.runWrite(ctx, "record_combination", func(txCtx context.Context) (ports.Stats, error) {
		if err := validateCombinationTimes(start, end); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.RecordCombination(txCtx, start, end)
		return stats, err
	})
	return result, err
}
func (s *Service) ReviseCombination(ctx context.Context, id int64, start time.Time, end *time.Time) (result ports.CombinedGroup, err error) {
	err = s.runWrite(ctx, "revise_combination", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid combination ID")
		}
		if err := validateCombinationTimes(start, end); err != nil {
			return ports.Stats{}, err
		}
		var stats ports.Stats
		result, stats, err = s.store.ReviseCombination(txCtx, id, start, end)
		return stats, err
	})
	return result, err
}
func (s *Service) DeleteCombination(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_combination", func(txCtx context.Context) (ports.Stats, error) {
		if id <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid combination ID")
		}
		return s.store.DeleteCombination(txCtx, id)
	})
}
func validateCombinationTimes(start time.Time, end *time.Time) error {
	if start.IsZero() {
		return errors.New("start time is required")
	}
	if end != nil && start.After(*end) {
		return errors.New("start time must be before end time")
	}
	return nil
}
