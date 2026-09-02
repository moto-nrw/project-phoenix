package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
)

// facilitiesPersonQuery adapts the People Directory to the narrow person
// port the facility service declares for supervisor names.
type facilitiesPersonQuery struct {
	persons peopledirectory.Query
}

func (q facilitiesPersonQuery) ListPersonsByID(ctx context.Context, ids []int64) ([]facilitiesService.Person, error) {
	persons, err := q.persons.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]facilitiesService.Person, 0, len(persons))
	for _, person := range persons {
		result = append(result, facilitiesService.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName})
	}
	return result, nil
}

func newFacilitiesPersonQuery(persons peopledirectory.Query) facilitiesService.PersonQuery {
	return facilitiesPersonQuery{persons: persons}
}

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
