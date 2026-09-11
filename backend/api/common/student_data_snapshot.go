package common

import (
	"context"
	"fmt"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

type snapshotPersonReader interface {
	GetByIDs(context.Context, []int64) (map[int64]*userModels.Person, error)
}

type snapshotGroupReader interface {
	GetGroupsByIDs(context.Context, []int64) (map[int64]*educationModels.Group, error)
}

// StudentDataSnapshot caches all data needed for building student list responses.
// This eliminates N+1 query problems by loading all related data in bulk.
type StudentDataSnapshot struct {
	Persons          map[int64]*userModels.Person
	Groups           map[int64]*educationModels.Group
	LocationSnapshot *StudentLocationSnapshot
}

// LoadStudentDataSnapshot batches all data needed to build student list responses.
// This prevents N+1 queries by loading persons, groups, and locations in bulk.
func LoadStudentDataSnapshot(
	ctx context.Context,
	personService snapshotPersonReader,
	educationSvc snapshotGroupReader,
	activeSvc activeService.Service,
	studentIDs []int64,
	personIDs []int64,
	groupIDs []int64,
) (*StudentDataSnapshot, error) {
	snapshot := &StudentDataSnapshot{
		Persons: make(map[int64]*userModels.Person),
		Groups:  make(map[int64]*educationModels.Group),
	}

	if err := loadSnapshotPersons(ctx, snapshot, personService, personIDs); err != nil {
		return nil, err
	}
	if err := loadSnapshotGroups(ctx, snapshot, educationSvc, groupIDs); err != nil {
		return nil, err
	}
	if err := loadSnapshotLocations(ctx, snapshot, activeSvc, studentIDs); err != nil {
		return nil, err
	}

	return snapshot, nil
}

func loadSnapshotPersons(ctx context.Context, snapshot *StudentDataSnapshot, svc snapshotPersonReader, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	persons, err := svc.GetByIDs(ctx, ids)
	if err == nil {
		snapshot.Persons = persons
		return nil
	}
	return fmt.Errorf("load student snapshot persons: %w", err)
}

func loadSnapshotGroups(ctx context.Context, snapshot *StudentDataSnapshot, svc snapshotGroupReader, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	groups, err := svc.GetGroupsByIDs(ctx, ids)
	if err == nil {
		snapshot.Groups = groups
		return nil
	}
	return fmt.Errorf("load student snapshot groups: %w", err)
}

func loadSnapshotLocations(ctx context.Context, snapshot *StudentDataSnapshot, svc activeService.Service, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	locations, err := LoadStudentLocationSnapshot(ctx, svc, ids)
	if err == nil {
		snapshot.LocationSnapshot = locations
		return nil
	}
	return fmt.Errorf("load student snapshot locations: %w", err)
}

// GetPerson retrieves a person from the snapshot with nil safety
func (s *StudentDataSnapshot) GetPerson(personID int64) *userModels.Person {
	if s == nil || s.Persons == nil {
		return nil
	}
	return s.Persons[personID]
}

// GetGroup retrieves a group from the snapshot with nil safety
func (s *StudentDataSnapshot) GetGroup(groupID int64) *educationModels.Group {
	if s == nil || s.Groups == nil {
		return nil
	}
	return s.Groups[groupID]
}

// ResolveLocationWithTime retrieves location info including entry time from the snapshot
func (s *StudentDataSnapshot) ResolveLocationWithTime(studentID int64, hasFullAccess bool) StudentLocationInfo {
	if s == nil || s.LocationSnapshot == nil {
		return StudentLocationInfo{Location: "Abwesend"}
	}
	return s.LocationSnapshot.ResolveStudentLocationWithTime(studentID, hasFullAccess)
}
