// backend/services/schedule/schedule_service.go
package schedule

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// Operation name constants to avoid string duplication
const (
	opGenerateEvents     = "generate events"
	opFindAvailableSlots = "find available slots"
)

// service implements the schedule.Service interface
type service struct {
	dateframeRepo      schedule.DateframeRepository
	timeframeRepo      schedule.TimeframeRepository
	recurrenceRuleRepo schedule.RecurrenceRuleRepository
	db                 *bun.DB
	txHandler          *base.TxHandler
}

// NewService creates a new schedule service
func NewService(
	dateframeRepo schedule.DateframeRepository,
	timeframeRepo schedule.TimeframeRepository,
	recurrenceRuleRepo schedule.RecurrenceRuleRepository,
	db *bun.DB,
) Service {
	return &service{
		dateframeRepo:      dateframeRepo,
		timeframeRepo:      timeframeRepo,
		recurrenceRuleRepo: recurrenceRuleRepo,
		db:                 db,
		txHandler:          base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var dateframeRepo = s.dateframeRepo
	var timeframeRepo = s.timeframeRepo
	var recurrenceRuleRepo = s.recurrenceRuleRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.dateframeRepo.(base.TransactionalRepository); ok {
		dateframeRepo = txRepo.WithTx(tx).(schedule.DateframeRepository)
	}
	if txRepo, ok := s.timeframeRepo.(base.TransactionalRepository); ok {
		timeframeRepo = txRepo.WithTx(tx).(schedule.TimeframeRepository)
	}
	if txRepo, ok := s.recurrenceRuleRepo.(base.TransactionalRepository); ok {
		recurrenceRuleRepo = txRepo.WithTx(tx).(schedule.RecurrenceRuleRepository)
	}

	// Return a new service with the transaction
	return &service{
		dateframeRepo:      dateframeRepo,
		timeframeRepo:      timeframeRepo,
		recurrenceRuleRepo: recurrenceRuleRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
	}
}

// Timeframe operations

// GetTimeframe retrieves a timeframe by its ID
func (s *service) GetTimeframe(ctx context.Context, id int64) (*schedule.Timeframe, error) {
	timeframe, err := s.timeframeRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &ScheduleError{Op: "get timeframe", Err: ErrTimeframeNotFound}
	}
	return timeframe, nil
}

// CreateTimeframe creates a new timeframe
func (s *service) CreateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	if err := timeframe.Validate(); err != nil {
		return &ScheduleError{Op: "create timeframe", Err: err}
	}

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

	if err := s.timeframeRepo.Update(ctx, timeframe); err != nil {
		return &ScheduleError{Op: "update timeframe", Err: err}
	}

	return nil
}

// DeleteTimeframe deletes a timeframe by its ID
func (s *service) DeleteTimeframe(ctx context.Context, id int64) error {
	if err := s.timeframeRepo.Delete(ctx, id); err != nil {
		return &ScheduleError{Op: "delete timeframe", Err: err}
	}

	return nil
}

// ListTimeframes retrieves all timeframes matching the provided filters
func (s *service) ListTimeframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Timeframe, error) {
	timeframes, err := s.timeframeRepo.List(ctx, options)
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
		return nil, &ScheduleError{Op: "get recurrence rule", Err: ErrRecurrenceRuleNotFound}
	}
	return rule, nil
}

// CreateRecurrenceRule creates a new recurrence rule
func (s *service) CreateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	if err := rule.Validate(); err != nil {
		return &ScheduleError{Op: "create recurrence rule", Err: err}
	}

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
	rules, err := s.recurrenceRuleRepo.List(ctx, options)
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

// GenerateEvents generates events based on a recurrence rule within a date range
func (s *service) GenerateEvents(ctx context.Context, ruleID int64, startDate, endDate time.Time) ([]time.Time, error) {
	// Get the recurrence rule
	rule, err := s.recurrenceRuleRepo.FindByID(ctx, ruleID)
	if err != nil {
		return nil, &ScheduleError{Op: opGenerateEvents, Err: ErrRecurrenceRuleNotFound}
	}

	// Validate date range
	if startDate.After(endDate) {
		return nil, &ScheduleError{Op: opGenerateEvents, Err: ErrInvalidDateRange}
	}

	// Check if rule has an end date that precedes startDate
	if rule.EndDate != nil && rule.EndDate.Before(startDate) {
		return []time.Time{}, nil // Rule doesn't apply to this range
	}

	// Adjust endDate if rule has an earlier end date
	if rule.EndDate != nil && rule.EndDate.Before(endDate) {
		endDate = *rule.EndDate
	}

	// Generate the events based on the rule's frequency
	var events []time.Time

	switch rule.Frequency {
	case schedule.FrequencyDaily:
		events = s.generateDailyEvents(rule, startDate, endDate)
	case schedule.FrequencyWeekly:
		events = s.generateWeeklyEvents(rule, startDate, endDate)
	case schedule.FrequencyMonthly:
		events = s.generateMonthlyEvents(rule, startDate, endDate)
	case schedule.FrequencyYearly:
		events = s.generateYearlyEvents(rule, startDate, endDate)
	default:
		return nil, &ScheduleError{Op: opGenerateEvents, Err: fmt.Errorf("unsupported frequency: %s", rule.Frequency)}
	}

	// If count is specified, limit the number of events
	if rule.Count != nil && len(events) > *rule.Count {
		events = events[:*rule.Count]
	}

	return events, nil
}

// Helper functions for GenerateEvents

// generateDailyEvents generates daily events based on the rule
func (s *service) generateDailyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
	var events []time.Time

	// Normalize times to start of day if needed
	currentDate := startDate

	for !currentDate.After(endDate) {
		events = append(events, currentDate)
		// Advance by interval count days
		currentDate = currentDate.AddDate(0, 0, rule.IntervalCount)
	}

	return events
}

// generateWeeklyEvents generates weekly events based on the rule
func (s *service) generateWeeklyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
	var events []time.Time

	// If no weekdays specified, use the weekday of the start date
	weekdays := rule.Weekdays
	if len(weekdays) == 0 {
		// Get the weekday of the start date
		weekdayNum := int(startDate.Weekday())
		if weekdayNum == 0 { // Sunday
			weekdayNum = 7
		}
		weekday := []string{"", "MON", "TUE", "WED", "THU", "FRI", "SAT", "SUN"}[weekdayNum]
		weekdays = []string{weekday}
	}

	// Map weekday strings to time.Weekday values
	weekdayValues := map[string]time.Weekday{
		"MON": time.Monday,
		"TUE": time.Tuesday,
		"WED": time.Wednesday,
		"THU": time.Thursday,
		"FRI": time.Friday,
		"SAT": time.Saturday,
		"SUN": time.Sunday,
	}

	// Start from the start date
	currentDate := startDate

	// Loop until we pass the end date
	for !currentDate.After(endDate) {
		currentWeekday := currentDate.Weekday()

		// Check if the current day is one of the rule's weekdays
		for _, wd := range weekdays {
			if weekdayValues[wd] == currentWeekday {
				events = append(events, currentDate)
				break
			}
		}

		// Advance to the next day
		currentDate = currentDate.AddDate(0, 0, 1)

		// If we've advanced to a new week, apply the interval count
		if currentDate.Weekday() == time.Monday && rule.IntervalCount > 1 {
			currentDate = currentDate.AddDate(0, 0, 7*(rule.IntervalCount-1))
		}
	}

	return events
}

// generateMonthlyEvents generates monthly events based on the rule
func (s *service) generateMonthlyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
	var events []time.Time

	// If no month days specified, use the day of month of the start date
	monthDays := rule.MonthDays
	if len(monthDays) == 0 {
		monthDays = []int{startDate.Day()}
	}

	// Start from the first day of the start date's month
	currentYear := startDate.Year()
	currentMonth := startDate.Month()
	startHour, startMin, startSec := startDate.Clock()

	// Loop through months until we pass the end date
	for {
		// For each specified day of the month
		for _, day := range monthDays {
			// Check if the day is valid for this month
			lastDayOfMonth := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, startDate.Location()).Day()
			if day > lastDayOfMonth {
				day = lastDayOfMonth // Use the last day if specified day exceeds month length
			}

			// Create the event date
			eventDate := time.Date(currentYear, currentMonth, day, startHour, startMin, startSec, 0, startDate.Location())

			// Add the event if it falls within our range
			if !eventDate.Before(startDate) && !eventDate.After(endDate) {
				events = append(events, eventDate)
			}
		}

		// Advance to the next month based on interval
		currentMonth += time.Month(rule.IntervalCount)
		for currentMonth > 12 {
			currentMonth -= 12
			currentYear++
		}

		// Check if we've gone past the end date
		if time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, startDate.Location()).After(endDate) {
			break
		}
	}

	return events
}

// generateYearlyEvents generates yearly events based on the rule
func (s *service) generateYearlyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
	var events []time.Time

	// Use start date's month and day for yearly events
	startMonth := startDate.Month()
	startDay := startDate.Day()
	startHour, startMin, startSec := startDate.Clock()

	// Iterate through years from start year to end year
	for year := startDate.Year(); year <= endDate.Year(); year += rule.IntervalCount {
		// Check if it's a leap year issue with February 29
		maxDay := startDay
		if startMonth == time.February && startDay == 29 {
			isLeapYear := (year%4 == 0 && year%100 != 0) || year%400 == 0
			if !isLeapYear {
				maxDay = 28
			}
		}

		// Create the event date
		eventDate := time.Date(year, startMonth, maxDay, startHour, startMin, startSec, 0, startDate.Location())

		// Add the event if it falls within our range
		if !eventDate.Before(startDate) && !eventDate.After(endDate) {
			events = append(events, eventDate)
		}
	}

	return events
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

	// Get all timeframes within the date range
	existingTimeframes, err := s.timeframeRepo.FindByTimeRange(ctx, startDate, endDate)
	if err != nil {
		return nil, &ScheduleError{Op: opFindAvailableSlots, Err: err}
	}

	// Sort timeframes by start time
	sort.Slice(existingTimeframes, func(i, j int) bool {
		return existingTimeframes[i].StartTime.Before(existingTimeframes[j].StartTime)
	})

	// Find available slots
	var availableSlots []*schedule.Timeframe
	currentTime := startDate

	for _, tf := range existingTimeframes {
		// If there's a gap before this timeframe, add it as an available slot
		if currentTime.Before(tf.StartTime) {
			endSlotTime := tf.StartTime
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
			currentTime = *tf.EndTime
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
