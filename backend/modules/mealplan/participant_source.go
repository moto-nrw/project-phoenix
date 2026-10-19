package mealplan

import "context"

type ParticipantCandidate struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

type ParticipantFinder func(context.Context, string) ([]ParticipantCandidate, error)
