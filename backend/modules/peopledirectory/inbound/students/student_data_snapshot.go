package students

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// studentDataSnapshot caches what a student list renders besides the rows:
// the persons they render under and their locations, loaded in bulk so a
// page never reads per child.
type studentDataSnapshot struct {
	Persons          map[int64]*peopleModule.Person
	LocationSnapshot *common.StudentLocationSnapshot
}

// loadStudentDataSnapshot batches the persons (People Directory) and the
// locations (Student Presence) of a page.
func (rs *Resource) loadStudentDataSnapshot(ctx context.Context, studentIDs, personIDs []int64) (*studentDataSnapshot, error) {
	snapshot := &studentDataSnapshot{Persons: map[int64]*peopleModule.Person{}}
	if len(personIDs) > 0 {
		persons, err := rs.personsByIDs(ctx, personIDs)
		if err != nil {
			return nil, fmt.Errorf("load student snapshot persons: %w", err)
		}
		snapshot.Persons = persons
	}
	if len(studentIDs) > 0 {
		locations, err := common.LoadStudentLocationSnapshot(ctx, rs.ActiveService, studentIDs)
		if err != nil {
			return nil, fmt.Errorf("load student snapshot locations: %w", err)
		}
		snapshot.LocationSnapshot = locations
	}
	return snapshot, nil
}

// GetPerson retrieves a person from the snapshot with nil safety.
func (s *studentDataSnapshot) GetPerson(personID int64) *peopleModule.Person {
	if s == nil || s.Persons == nil {
		return nil
	}
	return s.Persons[personID]
}

// ResolveLocationWithTime retrieves location info including entry time.
func (s *studentDataSnapshot) ResolveLocationWithTime(studentID int64, hasFullAccess bool) common.StudentLocationInfo {
	if s == nil || s.LocationSnapshot == nil {
		return common.StudentLocationInfo{Location: "Abwesend"}
	}
	return s.LocationSnapshot.ResolveStudentLocationWithTime(studentID, hasFullAccess)
}
