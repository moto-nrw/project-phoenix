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

// CalendarFinder reads the school calendar for the inclusive window
// [from, to]: the closed days (closing days, public holidays) and the days of
// school holiday periods, both as YYYY-MM-DD sets.
type CalendarFinder func(ctx context.Context, from, to string) (closed, holiday map[string]bool, err error)
