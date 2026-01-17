package schedule

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	schedulePort "github.com/moto-nrw/project-phoenix/internal/core/port/schedule"
)

// recurrenceHandler handles recurrence rule-specific operations
type recurrenceHandler struct {
	repo schedulePort.RecurrenceRuleRepository
}

// newRecurrenceHandler creates a new recurrence rule handler
func newRecurrenceHandler(repo schedulePort.RecurrenceRuleRepository) *recurrenceHandler {
	return &recurrenceHandler{repo: repo}
}

// GetRecurrenceRule retrieves a recurrence rule by its ID
func (h *recurrenceHandler) GetRecurrenceRule(ctx context.Context, id int64) (*schedule.RecurrenceRule, error) {
	rule, err := h.repo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get recurrence rule", Err: ErrRecurrenceRuleNotFound}
	}
	return rule, nil
}

// CreateRecurrenceRule creates a new recurrence rule
func (h *recurrenceHandler) CreateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return &ScheduleError{Op: "create recurrence rule", Err: err}
	}

	if err := h.repo.Create(ctx, rule); err != nil {
		return &ScheduleError{Op: "create recurrence rule", Err: err}
	}

	return nil
}

// UpdateRecurrenceRule updates an existing recurrence rule
func (h *recurrenceHandler) UpdateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return &ScheduleError{Op: "update recurrence rule", Err: err}
	}

	if err := h.repo.Update(ctx, rule); err != nil {
		return &ScheduleError{Op: "update recurrence rule", Err: err}
	}

	return nil
}

// DeleteRecurrenceRule deletes a recurrence rule by its ID
func (h *recurrenceHandler) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	if err := h.repo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete recurrence rule", Err: err}
	}

	return nil
}

// ListRecurrenceRules retrieves all recurrence rules matching the provided filters
func (h *recurrenceHandler) ListRecurrenceRules(ctx context.Context, options *base.QueryOptions) ([]*schedule.RecurrenceRule, error) {
	rules, err := h.repo.List(ctx, options)
	if err != nil {
		return nil, &ScheduleError{Op: "list recurrence rules", Err: err}
	}

	return rules, nil
}

// FindRecurrenceRulesByFrequency finds all recurrence rules with the specified frequency
func (h *recurrenceHandler) FindRecurrenceRulesByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error) {
	rules, err := h.repo.FindByFrequency(ctx, frequency)
	if err != nil {
		return nil, &ScheduleError{Op: "find recurrence rules by frequency", Err: err}
	}

	return rules, nil
}

// FindRecurrenceRulesByWeekday finds all recurrence rules that include the specified weekday
func (h *recurrenceHandler) FindRecurrenceRulesByWeekday(ctx context.Context, weekday string) ([]*schedule.RecurrenceRule, error) {
	rules, err := h.repo.FindByWeekday(ctx, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: "find recurrence rules by weekday", Err: err}
	}

	return rules, nil
}
