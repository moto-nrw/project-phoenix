package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

func (s *Service) FindRecurrenceRule(ctx context.Context, id int64) (result domain.RecurrenceRule, err error) {
	err = s.run("find_recurrence_rule", func(stats *domain.OperationStats) error {
		value, found, queryStats, findErr := s.store.FindRecurrenceRule(ctx, id)
		stats.Add(queryStats)
		if findErr != nil {
			return findErr
		}
		if !found {
			return domain.ErrRecurrenceRuleNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) ListRecurrenceRules(ctx context.Context, filter domain.RecurrenceRuleFilter) (result []domain.RecurrenceRule, err error) {
	err = s.run("list_recurrence_rules", func(stats *domain.OperationStats) error {
		values, queryStats, listErr := s.store.ListRecurrenceRules(ctx, filter)
		stats.Add(queryStats)
		result = values
		return listErr
	})
	return result, err
}

func (s *Service) CreateRecurrenceRule(ctx context.Context, fields domain.RecurrenceRuleFields) (result domain.RecurrenceRule, err error) {
	err = s.runWrite(ctx, "create_recurrence_rule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, queryStats, createErr := s.store.CreateRecurrenceRule(txCtx, fields)
		stats.Add(queryStats)
		result = value
		return createErr
	})
	return result, err
}

func (s *Service) UpdateRecurrenceRule(ctx context.Context, id int64, fields domain.RecurrenceRuleFields) (result domain.RecurrenceRule, err error) {
	err = s.runWrite(ctx, "update_recurrence_rule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		value, found, queryStats, updateErr := s.store.UpdateRecurrenceRule(txCtx, id, fields)
		stats.Add(queryStats)
		if updateErr != nil {
			return updateErr
		}
		if !found {
			return domain.ErrRecurrenceRuleNotFound
		}
		result = value
		return nil
	})
	return result, err
}

func (s *Service) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_recurrence_rule", true, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteRecurrenceRule(txCtx, id)
		stats.Add(queryStats)
		return err
	})
}
