package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
)

// The People Directory read ports the root binds for the room snapshot
// export and the Tagesinformationen (#3771, #2208), over the retained
// person service and the owner query. The consumers own the port shapes.

// personNameSource is the slice of the person service that resolves names.
type personNameSource interface {
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.DirectoryPersonSnapshot, error)
}

// roomSnapshotSource is the slice of the person service the room snapshot
// export reads.
type roomSnapshotSource interface {
	personNameSource
	GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.StudentState, error)
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
			result[id] = application.ToDirectoryStudent(student)
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
