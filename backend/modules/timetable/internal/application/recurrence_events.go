package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

// GenerateRecurrenceEvents uses the owner rule and preserves the established
// daily/weekly/monthly/yearly event ordering, clamping, and count semantics.
func (s *Service) GenerateRecurrenceEvents(ctx context.Context, id int64, start, end time.Time) (events []time.Time, err error) {
	err = s.run("generate_recurrence_events", func(stats *domain.OperationStats) error {
		rule, found, measured, readErr := s.store.FindRecurrenceRule(ctx, id)
		stats.Add(measured)
		if readErr != nil {
			return readErr
		}
		if !found {
			return domain.ErrRecurrenceRuleNotFound
		}
		if start.After(end) {
			return domain.ErrInvalidRecurrenceRange
		}
		if rule.IntervalCount <= 0 || (rule.Count != nil && *rule.Count < 0) {
			return fmt.Errorf("invalid stored recurrence interval or count")
		}
		if rule.EndDate != nil && rule.EndDate.Before(start) {
			events = []time.Time{}
			return nil
		}
		if rule.EndDate != nil && rule.EndDate.Before(end) {
			end = *rule.EndDate
		}
		switch rule.Frequency {
		case "daily":
			events = generateDailyEvents(&rule, start, end)
		case "weekly":
			events = generateWeeklyEvents(&rule, start, end)
		case "monthly":
			events = generateMonthlyEvents(&rule, start, end)
		case "yearly":
			events = generateYearlyEvents(&rule, start, end)
		default:
			return fmt.Errorf("unsupported frequency: %s", rule.Frequency)
		}
		if rule.Count != nil && len(events) > *rule.Count {
			events = events[:*rule.Count]
		}
		return nil
	})
	return events, err
}

// Helper functions for GenerateEvents

// generateDailyEvents generates daily events based on the rule
func generateDailyEvents(rule *domain.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
func generateWeeklyEvents(rule *domain.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
func generateMonthlyEvents(rule *domain.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
func generateYearlyEvents(rule *domain.RecurrenceRule, startDate, endDate time.Time) []time.Time {
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
