package mealplan

import "context"

type ParticipantCandidate struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

type ParticipantFinder func(context.Context, string) ([]ParticipantCandidate, error)

// StudentNameFinder names children by ID, enrolled or not, for past orders.
type StudentNameFinder func(context.Context, []int64) ([]ParticipantCandidate, error)

// HolidayPeriod is one school holiday period as the school calendar keeps
// it: YYYY-MM-DD dates, both ends inclusive.
type HolidayPeriod struct {
	Start string
	End   string
}

// CalendarFinder reads the school calendar for the inclusive window
// [from, to]: the closed days (closing days, public holidays) as a
// YYYY-MM-DD set and the school holiday periods that touch the window. The
// Meal Plan turns the periods into days itself.
type CalendarFinder func(ctx context.Context, from, to string) (closed map[string]bool, holidays []HolidayPeriod, err error)
