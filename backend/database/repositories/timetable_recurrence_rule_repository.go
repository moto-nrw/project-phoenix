package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

type timetableRecurrenceRuleRepository struct {
	timetable timetable.RecurrenceRuleCapability
}

func (r timetableRecurrenceRuleRepository) Create(ctx context.Context, value *scheduleModels.RecurrenceRule) error {
	if value == nil {
		return errors.New("recurrence rule cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateRecurrenceRule(ctx, publicRecurrenceRuleInput(value))
	if err != nil {
		return timetableCompose.WrapDatabaseError("create", err)
	}
	replaceLegacyRecurrenceRule(value, created)
	return nil
}

func (r timetableRecurrenceRuleRepository) FindByID(ctx context.Context, id any) (*scheduleModels.RecurrenceRule, error) {
	ruleID, ok := legacyGroupID(id)
	if !ok {
		return nil, timetableCompose.WrapDatabaseError("find by id", fmt.Errorf("invalid recurrence rule id %T", id))
	}
	value, err := r.timetable.FindRecurrenceRule(ctx, ruleID)
	if err != nil {
		return nil, legacyRecurrenceRuleError("find by id", err)
	}
	return legacyRecurrenceRule(value), nil
}

func (r timetableRecurrenceRuleRepository) Update(ctx context.Context, value *scheduleModels.RecurrenceRule) error {
	if value == nil {
		return errors.New("recurrence rule cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateRecurrenceRule(ctx, value.ID, publicRecurrenceRuleInput(value))
	if err != nil {
		return legacyRecurrenceRuleError("update", err)
	}
	replaceLegacyRecurrenceRule(value, updated)
	return nil
}

func (r timetableRecurrenceRuleRepository) Delete(ctx context.Context, id any) error {
	ruleID, ok := legacyGroupID(id)
	if !ok {
		return timetableCompose.WrapDatabaseError("delete", fmt.Errorf("invalid recurrence rule id %T", id))
	}
	if err := r.timetable.DeleteRecurrenceRule(ctx, ruleID); err != nil {
		return timetableCompose.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableRecurrenceRuleRepository) List(ctx context.Context, options *timetableCompose.RecurrenceRuleQueryOptions) ([]*scheduleModels.RecurrenceRule, error) {
	frequency, frequencies, sortBy, descending, limit, offset, err := timetableCompose.RecurrenceRuleListOptions(options)
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("list", err)
	}
	return r.list(ctx, timetable.RecurrenceRuleFilter{Frequency: frequency, Frequencies: frequencies,
		SortBy: sortBy, SortDescending: descending, Limit: limit, Offset: offset}, "list")
}

func (r timetableRecurrenceRuleRepository) FindByDateRange(ctx context.Context, startDate, _ time.Time) ([]*scheduleModels.RecurrenceRule, error) {
	return r.list(ctx, timetable.RecurrenceRuleFilter{ActiveAt: &startDate}, "find by date range")
}

func (r timetableRecurrenceRuleRepository) list(ctx context.Context, filter timetable.RecurrenceRuleFilter, operation string) ([]*scheduleModels.RecurrenceRule, error) {
	values, err := r.timetable.ListRecurrenceRules(ctx, filter)
	if err != nil {
		return nil, legacyRecurrenceRuleError(operation, err)
	}
	result := make([]*scheduleModels.RecurrenceRule, 0, len(values))
	for _, value := range values {
		result = append(result, legacyRecurrenceRule(value))
	}
	return result, nil
}

func legacyRecurrenceRule(value timetable.RecurrenceRule) *scheduleModels.RecurrenceRule {
	result := &scheduleModels.RecurrenceRule{Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndDate: value.EndDate, Count: value.Count}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	result.SetTenantID(value.TenantID)
	return result
}

func replaceLegacyRecurrenceRule(result *scheduleModels.RecurrenceRule, value timetable.RecurrenceRule) {
	*result = *legacyRecurrenceRule(value)
}

func publicRecurrenceRuleInput(value *scheduleModels.RecurrenceRule) timetable.RecurrenceRuleInput {
	return timetable.RecurrenceRuleInput{Frequency: value.Frequency, IntervalCount: value.IntervalCount,
		Weekdays: value.Weekdays, MonthDays: value.MonthDays, EndDate: value.EndDate, Count: value.Count}
}

func legacyRecurrenceRuleError(operation string, err error) error {
	if errors.Is(err, timetable.ErrRecurrenceRuleNotFound) {
		return timetableCompose.WrapNotFoundDatabaseError(operation)
	}
	return timetableCompose.WrapDatabaseError(operation, err)
}
