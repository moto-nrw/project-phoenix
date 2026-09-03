package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
)

// educationPersonQuery adapts the People Directory to the person port the
// substitution module declares for staff names.
type educationPersonQuery struct {
	persons peopledirectory.Query
}

func (q educationPersonQuery) ListPersonsByID(ctx context.Context, ids []int64) ([]educationService.Person, error) {
	persons, err := q.persons.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]educationService.Person, 0, len(persons))
	for _, person := range persons {
		result = append(result, educationService.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName})
	}
	return result, nil
}

func newEducationPersonQuery(persons peopledirectory.Query) educationService.PersonQuery {
	return educationPersonQuery{persons: persons}
}
