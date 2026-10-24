package api

import (
	"context"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/grouplive"
)

// The People Directory read ports of the room snapshot export and the live
// group roster, bound over the retained person service (#3771). The consumers
// own the port shapes; these adapters translate the retained rows and change
// nothing about which reads run.

// groupRosterSource is the slice of the person service the live group roster
// reads.
type groupRosterSource interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
	GetParticipationCandidatesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*peopleValues.StudentState, error)
}

type groupRosterPeople struct{ source groupRosterSource }

// GroupRosterPeople serves the live group roster's People port from the
// person service.
func GroupRosterPeople(source groupRosterSource) GroupRoster {
	if source == nil {
		return nil
	}
	return groupRosterPeople{source: source}
}

// GroupMembers returns the participation candidates of the group with their
// person names; members without a person row are omitted.
func (p groupRosterPeople) GroupMembers(ctx context.Context, groupID int64) ([]grouplive.RosterStudent, error) {
	students, err := p.source.GetParticipationCandidatesByGroupIDs(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return []grouplive.RosterStudent{}, nil
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := p.source.GetByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("bulk load persons: %w", err)
	}
	members := make([]grouplive.RosterStudent, 0, len(students))
	for _, student := range students {
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		member := grouplive.RosterStudent{
			ID: student.ID, FirstName: person.FirstName, LastName: person.LastName,
			SchoolClass: student.SchoolClass, SickSince: student.SickSince, ExcusedSince: student.ExcusedSince,
			PhotoPath: student.PhotoPath,
		}
		if student.Sick != nil {
			member.Sick = *student.Sick
		}
		if student.Excused != nil {
			member.Excused = *student.Excused
		}
		members = append(members, member)
	}
	return members, nil
}
