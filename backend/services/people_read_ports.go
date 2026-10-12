package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The People Directory read ports of the room snapshot export and the live
// group roster, bound over the retained person service (#3771). The consumers
// own the port shapes; these adapters translate the retained rows and change
// nothing about which reads run.

// personNameSource is the slice of the person service that resolves names.
type personNameSource interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error)
}

// roomSnapshotSource is the slice of the person service the room snapshot
// export reads.
type roomSnapshotSource interface {
	personNameSource
	GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*users.Student, error)
}

// RoomSnapshotPeople serves the Students port of the room snapshot export
// from the person service.
type RoomSnapshotPeople struct{ source roomSnapshotSource }

// NewRoomSnapshotPeople binds the room snapshot's Students port to the person
// service.
func NewRoomSnapshotPeople(source roomSnapshotSource) RoomSnapshotPeople {
	if source == nil {
		panic("room snapshot people: person service is required")
	}
	return RoomSnapshotPeople{source: source}
}

// StudentsByIDs returns the students for ids, keyed by student id.
func (p RoomSnapshotPeople) StudentsByIDs(ctx context.Context, ids []int64) (map[int64]peopledirectory.Student, error) {
	students, err := p.source.GetStudentsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]peopledirectory.Student, len(students))
	for id, student := range students {
		if student != nil {
			result[id] = toDirectoryStudent(student)
		}
	}
	return result, nil
}

// PersonsByIDs returns the names of the persons for ids, keyed by person id;
// unknown persons are absent.
func (p RoomSnapshotPeople) PersonsByIDs(ctx context.Context, ids []int64) (map[int64]peopledirectory.Person, error) {
	persons, err := p.source.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]peopledirectory.Person, len(persons))
	for id, person := range persons {
		if person != nil {
			result[id] = peopledirectory.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return result, nil
}

// groupRosterSource is the slice of the person service the live group roster
// reads.
type groupRosterSource interface {
	personNameSource
	GetParticipationCandidatesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*users.Student, error)
}

type groupRosterPeople struct{ source groupRosterSource }

// GroupRosterPeople serves the live group roster's People port from the
// person service.
func GroupRosterPeople(source groupRosterSource) grouplivelegacy.People {
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
