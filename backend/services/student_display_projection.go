package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type studentDisplayProjection struct {
	students peopledirectory.Query
	groups   schoolstructure.Query
}

func (q studentDisplayProjection) ListStudentDisplayFacts(ctx context.Context, ids []int64) ([]active.StudentDisplayFacts, error) {
	students, err := q.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	personIDs := make([]int64, 0, len(students))
	groupIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil && *student.GroupID > 0 {
			groupIDs = append(groupIDs, *student.GroupID)
		}
	}
	persons, err := q.students.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	personsByID := make(map[int64]peopledirectory.Person, len(persons))
	for _, person := range persons {
		personsByID[person.ID] = person
	}
	groups, err := q.groups.ListGroupsByID(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	groupNames := make(map[int64]string, len(groups))
	for _, group := range groups {
		groupNames[group.ID] = group.Name
	}
	result := make([]active.StudentDisplayFacts, 0, len(students))
	for _, student := range students {
		person := personsByID[student.PersonID]
		groupName := ""
		if student.GroupID != nil {
			groupName = groupNames[*student.GroupID]
		}
		result = append(result, active.StudentDisplayFacts{
			FirstName: person.FirstName, LastName: person.LastName, OGSGroupName: groupName,
			ID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID,
			Sick: student.Sick, SickSince: student.SickSince, Excused: student.Excused, ExcusedSince: student.ExcusedSince, PhotoPath: student.PhotoPath,
		})
	}
	return result, nil
}
