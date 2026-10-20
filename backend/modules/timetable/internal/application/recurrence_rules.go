package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindRecurrenceRule(ctx context.Context, id int64) (result timetable.RecurrenceRule, err error) {
	if id <= 0 {
		return timetable.RecurrenceRule{}, s.reject("find_recurrence_rule", timetable.ErrInvalidRecurrenceRuleQuery)
	}

	err = s.run(ctx, "find_recurrence_rule", func(ctx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindRecurrenceRule(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return timetable.ErrRecurrenceRuleNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListRecurrenceRules(ctx context.Context, filter timetable.RecurrenceRuleFilter) (result []timetable.RecurrenceRule, err error) {
	if filter.Limit < 0 || filter.Offset < 0 || !validRecurrenceRuleSort(filter.SortBy) {
		return nil, s.reject("list_recurrence_rules", timetable.ErrInvalidRecurrenceRuleQuery)
	}

	err = s.run(ctx, "list_recurrence_rules", func(ctx context.Context, stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListRecurrenceRules(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateRecurrenceRule(ctx context.Context, fields timetable.RecurrenceRuleInput) (result timetable.RecurrenceRule, err error) {
	fields, ok := normalizeRecurrenceRule(fields)
	if !ok {
		return timetable.RecurrenceRule{}, s.reject("create_recurrence_rule", timetable.ErrInvalidRecurrenceRule)
	}

	err = s.runWrite(ctx, "create_recurrence_rule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateRecurrenceRule(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateRecurrenceRule(ctx context.Context, id int64, fields timetable.RecurrenceRuleInput) (result timetable.RecurrenceRule, err error) {
	fields, ok := normalizeRecurrenceRule(fields)
	if id <= 0 || !ok {
		return timetable.RecurrenceRule{}, s.reject("update_recurrence_rule", timetable.ErrInvalidRecurrenceRule)
	}

	err = s.runWrite(ctx, "update_recurrence_rule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateRecurrenceRule(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return timetable.ErrRecurrenceRuleNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	if id <= 0 {
		return s.reject("delete_recurrence_rule", timetable.ErrInvalidRecurrenceRule)
	}

	return s.runWrite(ctx, "delete_recurrence_rule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteRecurrenceRule(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}
