package schedule

import (
	"context"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	schedulePort "github.com/moto-nrw/project-phoenix/internal/core/port/schedule"
)

// analysisHandler handles schedule analysis and event generation operations
type analysisHandler struct {
	timeframeRepo   schedulePort.TimeframeRepository
	recurrenceRepo  schedulePort.RecurrenceRuleRepository
	eventGenerator  *eventGenerator
}

// newAnalysisHandler creates a new analysis handler
func newAnalysisHandler(
	timeframeRepo schedulePort.TimeframeRepository,
	recurrenceRepo schedulePort.RecurrenceRuleRepository,
) *analysisHandler {
	return &analysisHandler{
		timeframeRepo:  timeframeRepo,
		recurrenceRepo: recurrenceRepo,
		eventGenerator: newEventGenerator(),
	}
}

// GenerateEvents generates events based on a recurrence rule within a date range
func (h *analysisHandler) GenerateEvents(ctx context.Context, ruleID int64, startDate, endDate time.Time) ([]time.Time, error) {
	// Get the recurrence rule
	rule, err := h.recurrenceRepo.FindByID(ctx, ruleID)
	if err != nil {
		return nil, &ScheduleError{Op: "generate events", Err: ErrRecurrenceRuleNotFound}
	}

	// Validate date range
	if startDate.After(endDate) {
		return nil, &ScheduleError{Op: "generate events", Err: ErrInvalidDateRange}
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
	events := h.eventGenerator.GenerateByFrequency(rule, startDate, endDate)

	// If count is specified, limit the number of events
	if rule.Count != nil && len(events) > *rule.Count {
		events = events[:*rule.Count]
	}

	return events, nil
}

// CheckConflict checks if there are any conflicts for the given time range
func (h *analysisHandler) CheckConflict(ctx context.Context, startTime, endTime time.Time) (bool, []*schedule.Timeframe, error) {
	// Validate time range
	if !endTime.IsZero() && startTime.After(endTime) {
		return false, nil, &ScheduleError{Op: "check conflict", Err: ErrInvalidTimeRange}
	}

	// Find timeframes that overlap with the given range
	timeframes, err := h.timeframeRepo.FindByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return false, nil, &ScheduleError{Op: "check conflict", Err: err}
	}

	hasConflict := len(timeframes) > 0
	return hasConflict, timeframes, nil
}

// FindAvailableSlots finds available time slots within a date range
func (h *analysisHandler) FindAvailableSlots(ctx context.Context, startDate, endDate time.Time, duration time.Duration) ([]*schedule.Timeframe, error) {
	// Validate input
	if startDate.After(endDate) {
		return nil, &ScheduleError{Op: "find available slots", Err: ErrInvalidDateRange}
	}

	if duration <= 0 {
		return nil, &ScheduleError{Op: "find available slots", Err: ErrInvalidDuration}
	}

	// Get all timeframes within the date range
	existingTimeframes, err := h.timeframeRepo.FindByTimeRange(ctx, startDate, endDate)
	if err != nil {
		return nil, &ScheduleError{Op: "find available slots", Err: err}
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

// eventGenerator handles event generation logic
type eventGenerator struct{}

// newEventGenerator creates a new event generator
func newEventGenerator() *eventGenerator {
	return &eventGenerator{}
}

// GenerateByFrequency dispatches to appropriate frequency-specific generator
func (g *eventGenerator) GenerateByFrequency(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
	switch rule.Frequency {
	case schedule.FrequencyDaily:
		return g.generateDailyEvents(rule, startDate, endDate)
	case schedule.FrequencyWeekly:
		return g.generateWeeklyEvents(rule, startDate, endDate)
	case schedule.FrequencyMonthly:
		return g.generateMonthlyEvents(rule, startDate, endDate)
	case schedule.FrequencyYearly:
		return g.generateYearlyEvents(rule, startDate, endDate)
	default:
		return nil // Should not reach here if rule is validated
	}
}

// generateDailyEvents generates daily events based on the rule
func (g *eventGenerator) generateDailyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
	var events []time.Time
	currentDate := startDate

	for !currentDate.After(endDate) {
		events = append(events, currentDate)
		// Advance by interval count days
		currentDate = currentDate.AddDate(0, 0, rule.IntervalCount)
	}

	return events
}

// generateWeeklyEvents generates weekly events based on the rule
func (g *eventGenerator) generateWeeklyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
func (g *eventGenerator) generateMonthlyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
func (g *eventGenerator) generateYearlyEvents(rule *schedule.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
