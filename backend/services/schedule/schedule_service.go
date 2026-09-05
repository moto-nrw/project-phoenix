// backend/services/schedule/schedule_service.go
package schedule

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Operation name constants to avoid string duplication
const (
	opGenerateEvents     = "generate events"
	opFindAvailableSlots = "find available slots"
)

// service implements the schedule.Service interface
type service struct {
	recurrenceEvents                    RecurrenceEventQuery
	dateframeRepo                       schedule.DateframeRepository
	timeframeRepo                       schedule.TimeframeRepository
	recurrenceRuleRepo                  schedule.RecurrenceRuleRepository
	lockTemplateRecurrence              func(context.Context) error
	validateCareOfferingTimeframeChange func(context.Context, int64, *schedule.Timeframe) error
}

// RecurrenceEventQuery is the consumer-owned expansion port.
type RecurrenceEventQuery interface {
	GenerateRecurrenceEvents(context.Context, int64, time.Time, time.Time) ([]time.Time, error)
}

// ServiceConfig carries the cross-domain guard needed by timeframe deletion.
type ServiceConfig struct {
	RecurrenceEvents                    RecurrenceEventQuery
	DateframeRepo                       schedule.DateframeRepository
	TimeframeRepo                       schedule.TimeframeRepository
	RecurrenceRuleRepo                  schedule.RecurrenceRuleRepository
	LockTemplateRecurrence              func(context.Context) error
	ValidateCareOfferingTimeframeChange func(context.Context, int64, *schedule.Timeframe) error
}

// NewServiceWithConfig builds the schedule service with recurrence-aware
// timeframe deletion validation.
func NewServiceWithConfig(cfg ServiceConfig) Service {
	if cfg.RecurrenceEvents == nil {
		panic("schedule service: recurrence event query is required")
	}
	return &service{
		recurrenceEvents:                    cfg.RecurrenceEvents,
		dateframeRepo:                       cfg.DateframeRepo,
		timeframeRepo:                       cfg.TimeframeRepo,
		recurrenceRuleRepo:                  cfg.RecurrenceRuleRepo,
		lockTemplateRecurrence:              cfg.LockTemplateRecurrence,
		validateCareOfferingTimeframeChange: cfg.ValidateCareOfferingTimeframeChange,
	}
}

// Dateframe operations

// GetDateframe retrieves a dateframe by its ID
func (s *service) GetDateframe(ctx context.Context, id int64) (*schedule.Dateframe, error) {
	dateframe, err := s.dateframeRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			err = ErrDateframeNotFound
		}
		return nil, &ScheduleError{Op: "get dateframe", Err: err}
	}
	return dateframe, nil
}

// CreateDateframe creates a new dateframe
func (s *service) CreateDateframe(ctx context.Context, dateframe *schedule.Dateframe) error {
	if err := dateframe.Validate(); err != nil {
		return &ScheduleError{Op: "create dateframe", Err: err}
	}

	dateframe.SetTenantID(tenant.FromContext(ctx))
	if err := s.dateframeRepo.Create(ctx, dateframe); err != nil {
		return &ScheduleError{Op: "create dateframe", Err: err}
	}

	return nil
}

// UpdateDateframe updates an existing dateframe
func (s *service) UpdateDateframe(ctx context.Context, dateframe *schedule.Dateframe) error {
	if err := dateframe.Validate(); err != nil {
		return &ScheduleError{Op: "update dateframe", Err: err}
	}

	if err := s.dateframeRepo.Update(ctx, dateframe); err != nil {
		return &ScheduleError{Op: "update dateframe", Err: err}
	}

	return nil
}

// DeleteDateframe deletes a dateframe by its ID
func (s *service) DeleteDateframe(ctx context.Context, id int64) error {
	if err := s.dateframeRepo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete dateframe", Err: err}
	}

	return nil
}

// ListDateframes retrieves all dateframes matching the provided filters
func (s *service) ListDateframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Dateframe, error) {
	dateframes, err := legacyList[*schedule.Dateframe](ctx, s.dateframeRepo, options)
	if err != nil {
		return nil, &ScheduleError{Op: "list dateframes", Err: err}
	}

	return dateframes, nil
}

// FindDateframesByDate finds all dateframes that include the given date
func (s *service) FindDateframesByDate(ctx context.Context, date time.Time) ([]*schedule.Dateframe, error) {
	// This method doesn't require a transaction, so we can directly call the repository
	dateframes, err := s.dateframeRepo.FindByDate(ctx, date)
	if err != nil {
		return nil, &ScheduleError{Op: "find dateframes by date", Err: err}
	}

	return dateframes, nil
}

// FindOverlappingDateframes finds all dateframes that overlap with the given date range
func (s *service) FindOverlappingDateframes(ctx context.Context, startDate, endDate time.Time) ([]*schedule.Dateframe, error) {
	// Validate date range
	if startDate.After(endDate) {
		return nil, &ScheduleError{Op: "find overlapping dateframes", Err: ErrInvalidDateRange}
	}

	dateframes, err := s.dateframeRepo.FindOverlapping(ctx, startDate, endDate)
	if err != nil {
		return nil, &ScheduleError{Op: "find overlapping dateframes", Err: err}
	}

	return dateframes, nil
}

// Timeframe operations

// GetTimeframe retrieves a timeframe by its ID
func (s *service) GetTimeframe(ctx context.Context, id int64) (*schedule.Timeframe, error) {
	timeframe, err := s.timeframeRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			err = ErrTimeframeNotFound
		}
		return nil, &ScheduleError{Op: "get timeframe", Err: err}
	}
	return timeframe, nil
}

// CreateTimeframe creates a new timeframe
func (s *service) CreateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return &ScheduleError{Op: "create timeframe", Err: err}
	}

	timeframe.SetTenantID(tenant.FromContext(ctx))
	if err := s.timeframeRepo.Create(ctx, timeframe); err != nil {
		return &ScheduleError{Op: "create timeframe", Err: err}
	}

	return nil
}

// UpdateTimeframe updates an existing timeframe
func (s *service) UpdateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return &ScheduleError{Op: "update timeframe", Err: err}
	}
	if err := s.validateTimeframeCareOfferingChange(ctx, timeframe.ID, timeframe); err != nil {
		return err
	}

	if err := s.timeframeRepo.Update(ctx, timeframe); err != nil {
		return &ScheduleError{Op: "update timeframe", Err: err}
	}

	return nil
}

// DeleteTimeframe deletes a timeframe by its ID
func (s *service) DeleteTimeframe(ctx context.Context, id int64) error {
	if err := s.validateTimeframeCareOfferingChange(ctx, id, nil); err != nil {
		return err
	}
	if err := s.timeframeRepo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete timeframe", Err: err}
	}

	return nil
}

func (s *service) validateTimeframeCareOfferingChange(
	ctx context.Context,
	id int64,
	replacement *schedule.Timeframe,
) error {
	if s.validateCareOfferingTimeframeChange == nil {
		return nil
	}
	op := "delete timeframe"
	if replacement != nil {
		op = "update timeframe"
	}
	if s.lockTemplateRecurrence == nil {
		return &ScheduleError{Op: op + ": lock timetable recurrence", Err: errors.New("template recurrence lock is not configured")}
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return &ScheduleError{Op: op + ": lock timetable recurrence", Err: err}
	}
	if err := s.validateCareOfferingTimeframeChange(ctx, id, replacement); err != nil {
		if errors.Is(err, enrollmentModels.ErrCareOfferingInvalid) {
			return &ScheduleError{Op: op, Err: ErrTimeframeRequiredByCareOffering}
		}
		return &ScheduleError{Op: op + ": validate care offerings", Err: err}
	}
	return nil
}

// ListTimeframes retrieves all timeframes matching the provided filters
func (s *service) ListTimeframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Timeframe, error) {
	timeframes, err := legacyList[*schedule.Timeframe](ctx, s.timeframeRepo, options)
	if err != nil {
		return nil, &ScheduleError{Op: "list timeframes", Err: err}
	}

	return timeframes, nil
}

// FindActiveTimeframes finds all active timeframes
func (s *service) FindActiveTimeframes(ctx context.Context) ([]*schedule.Timeframe, error) {
	timeframes, err := s.timeframeRepo.FindActive(ctx)
	if err != nil {
		return nil, &ScheduleError{Op: "find active timeframes", Err: err}
	}

	return timeframes, nil
}

// FindTimeframesByTimeRange finds all timeframes that overlap with the given time range
func (s *service) FindTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*schedule.Timeframe, error) {
	// Validate time range
	if !endTime.IsZero() && startTime.After(endTime) {
		return nil, &ScheduleError{Op: "find timeframes by time range", Err: ErrInvalidTimeRange}
	}

	timeframes, err := s.timeframeRepo.FindByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return nil, &ScheduleError{Op: "find timeframes by time range", Err: err}
	}

	return timeframes, nil
}

// RecurrenceRule operations

// GetRecurrenceRule retrieves a recurrence rule by its ID
func (s *service) GetRecurrenceRule(ctx context.Context, id int64) (*schedule.RecurrenceRule, error) {
	rule, err := s.recurrenceRuleRepo.FindByID(ctx, id)
	if err != nil {
		if base.IsNoRows(err) {
			err = ErrRecurrenceRuleNotFound
		}
		return nil, &ScheduleError{Op: "get recurrence rule", Err: err}
	}
	return rule, nil
}

// CreateRecurrenceRule creates a new recurrence rule
func (s *service) CreateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return &ScheduleError{Op: "create recurrence rule", Err: err}
	}

	rule.SetTenantID(tenant.FromContext(ctx))
	if err := s.recurrenceRuleRepo.Create(ctx, rule); err != nil {
		return &ScheduleError{Op: "create recurrence rule", Err: err}
	}

	return nil
}

// UpdateRecurrenceRule updates an existing recurrence rule
func (s *service) UpdateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return &ScheduleError{Op: "update recurrence rule", Err: err}
	}

	if err := s.recurrenceRuleRepo.Update(ctx, rule); err != nil {
		return &ScheduleError{Op: "update recurrence rule", Err: err}
	}

	return nil
}

// DeleteRecurrenceRule deletes a recurrence rule by its ID
func (s *service) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	if err := s.recurrenceRuleRepo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete recurrence rule", Err: err}
	}

	return nil
}

// ListRecurrenceRules retrieves all recurrence rules matching the provided filters
func (s *service) ListRecurrenceRules(ctx context.Context, options *base.QueryOptions) ([]*schedule.RecurrenceRule, error) {
	rules, err := legacyList[*schedule.RecurrenceRule](ctx, s.recurrenceRuleRepo, options)
	if err != nil {
		return nil, &ScheduleError{Op: "list recurrence rules", Err: err}
	}

	return rules, nil
}

// FindRecurrenceRulesByFrequency finds all recurrence rules with the specified frequency
func (s *service) FindRecurrenceRulesByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error) {
	rules, err := s.recurrenceRuleRepo.FindByFrequency(ctx, frequency)
	if err != nil {
		return nil, &ScheduleError{Op: "find recurrence rules by frequency", Err: err}
	}

	return rules, nil
}

// FindRecurrenceRulesByWeekday finds all recurrence rules that include the specified weekday
func (s *service) FindRecurrenceRulesByWeekday(ctx context.Context, weekday string) ([]*schedule.RecurrenceRule, error) {
	rules, err := s.recurrenceRuleRepo.FindByWeekday(ctx, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: "find recurrence rules by weekday", Err: err}
	}

	return rules, nil
}

// Advanced operations

// GenerateEvents delegates recurrence expansion to its owner while retaining
// the established service error wrapper at legacy callers.
func (s *service) GenerateEvents(ctx context.Context, ruleID int64, startDate, endDate time.Time) ([]time.Time, error) {
	events, err := s.recurrenceEvents.GenerateRecurrenceEvents(ctx, ruleID, startDate, endDate)
	if err == nil {
		return events, nil
	}
	switch {
	case errors.Is(err, timetable.ErrRecurrenceRuleNotFound):
		err = ErrRecurrenceRuleNotFound
	case errors.Is(err, timetable.ErrInvalidRecurrenceRange):
		err = ErrInvalidDateRange
	}
	return nil, &ScheduleError{Op: opGenerateEvents, Err: err}
}

// CheckConflict checks if there are any conflicts for the given time range
func (s *service) CheckConflict(ctx context.Context, startTime, endTime time.Time) (bool, []*schedule.Timeframe, error) {
	// Validate time range
	if !endTime.IsZero() && startTime.After(endTime) {
		return false, nil, &ScheduleError{Op: "check conflict", Err: ErrInvalidTimeRange}
	}

	// Find timeframes that overlap with the given range
	timeframes, err := s.timeframeRepo.FindByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return false, nil, &ScheduleError{Op: "check conflict", Err: err}
	}

	hasConflict := len(timeframes) > 0
	return hasConflict, timeframes, nil
}

// FindAvailableSlots finds available time slots within a date range
func (s *service) FindAvailableSlots(ctx context.Context, startDate, endDate time.Time, duration time.Duration) ([]*schedule.Timeframe, error) {
	// Validate input
	if startDate.After(endDate) {
		return nil, &ScheduleError{Op: opFindAvailableSlots, Err: ErrInvalidDateRange}
	}

	if duration <= 0 {
		return nil, &ScheduleError{Op: opFindAvailableSlots, Err: ErrInvalidDuration}
	}

	startDate = timezone.NormalizeWallClock(startDate)
	endDate = timezone.NormalizeWallClock(endDate)

	// Get all timeframes within the date range
	existingTimeframes, err := s.timeframeRepo.FindByTimeRange(ctx, startDate, endDate)
	if err != nil {
		return nil, &ScheduleError{Op: opFindAvailableSlots, Err: err}
	}

	// Sort timeframes by start time
	sort.Slice(existingTimeframes, func(i, j int) bool {
		return timezone.NormalizeWallClock(existingTimeframes[i].StartTime).Before(timezone.NormalizeWallClock(existingTimeframes[j].StartTime))
	})

	// Find available slots
	var availableSlots []*schedule.Timeframe
	currentTime := startDate

	for _, tf := range existingTimeframes {
		tfStart := timezone.NormalizeWallClock(tf.StartTime)

		// If there's a gap before this timeframe, add it as an available slot
		if currentTime.Before(tfStart) {
			endSlotTime := tfStart
			slot := &schedule.Timeframe{
				StartTime: currentTime,
				EndTime:   &endSlotTime,
				IsActive:  true,
			}

			// Only add if the slot is long enough
			if slot.Duration() >= duration {
				availableSlots = append(availableSlots, slot)
			}
		}

		// Update current time to the end of this timeframe
		if tf.EndTime != nil {
			currentTime = timezone.NormalizeWallClock(*tf.EndTime)
		} else {
			// Open-ended timeframe, no more available slots
			return availableSlots, nil
		}
	}

	// Add a final slot if there's time left
	if currentTime.Before(endDate) {
		slot := &schedule.Timeframe{
			StartTime: currentTime,
			EndTime:   &endDate,
			IsActive:  true,
		}

		// Only add if the slot is long enough
		if slot.Duration() >= duration {
			availableSlots = append(availableSlots, slot)
		}
	}

	return availableSlots, nil
}

// GetCurrentDateframe gets the active dateframe for the current date
func (s *service) GetCurrentDateframe(ctx context.Context) (*schedule.Dateframe, error) {
	now := time.Now()

	dateframes, err := s.dateframeRepo.FindByDate(ctx, now)
	if err != nil {
		return nil, &ScheduleError{Op: "get current dateframe", Err: err}
	}

	if len(dateframes) == 0 {
		return nil, &ScheduleError{Op: "get current dateframe", Err: ErrDateframeNotFound}
	}

	// If multiple dateframes are active, prioritize by name or creation date
	// For now, just return the first one
	return dateframes[0], nil
}
