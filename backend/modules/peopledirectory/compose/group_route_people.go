package compose

import (
	"context"
	"fmt"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// GroupRoutePeople binds the People Directory port of the group routes
// (modules/schoolstructure/http, #2742) to the retained person directory. It
// only translates the retained rows into the owner's public types and changes
// nothing about which reads run.
type GroupRoutePeople struct {
	persons *PersonDirectory
}

// NewGroupRoutePeople binds the retained person directory.
func NewGroupRoutePeople(persons *PersonDirectory) GroupRoutePeople {
	return GroupRoutePeople{persons: persons}
}

// GroupStudents returns the children of a group.
func (p GroupRoutePeople) GroupStudents(ctx context.Context, groupID int64) ([]peopleModule.StudentRecord, error) {
	students, err := p.persons.GetStudentsByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	result := make([]peopleModule.StudentRecord, 0, len(students))
	for _, student := range students {
		if student != nil {
			result = append(result, studentRouteRecord(student))
		}
	}
	return result, nil
}

// CountStudentsByGroupIDs counts the children of each group.
func (p GroupRoutePeople) CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	return p.persons.CountStudentsByGroupIDs(ctx, groupIDs)
}

// FindPerson returns one person.
func (p GroupRoutePeople) FindPerson(ctx context.Context, personID int64) (peopleModule.Person, error) {
	person, err := p.persons.Get(ctx, personID)
	if err != nil {
		return peopleModule.Person{}, err
	}
	if person == nil {
		return peopleModule.Person{}, fmt.Errorf("person %d: %w", personID, peopleModule.ErrPersonNotFound)
	}
	return studentRoutePerson(person), nil
}

// ListPersonsByID returns the persons keyed by ID; an unknown ID is absent.
func (p GroupRoutePeople) ListPersonsByID(ctx context.Context, personIDs []int64) (map[int64]peopleModule.Person, error) {
	persons, err := p.persons.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]peopleModule.Person, len(persons))
	for id, person := range persons {
		if person != nil {
			result[id] = studentRoutePerson(person)
		}
	}
	return result, nil
}
